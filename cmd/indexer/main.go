package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"

	"github.com/script3/soroban-governor-backend/internal/db"
	"github.com/script3/soroban-governor-backend/internal/indexer"
	"github.com/sirupsen/logrus"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/network"
	"github.com/stellar/go/support/log"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func main() {
	ctx := context.Background()
	source := "indexer"

	slog.Info("Starting indexer service...")

	slog.Info("Loading config...")
	config, err := indexer.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "err", err)
		os.Exit(1)
	}
	slog.Info("Config loaded.", "db_type", config.DBType, "ledger_backend", config.LedgerBackendType)

	slog.Info("Setting up database...")
	// Create the database
	database, err := sql.Open(config.DBType, config.DBConnectionString)
	if err != nil {
		log.Fatal(err)
	}
	database.SetMaxOpenConns(config.DBMaxOpenConns)
	database.SetMaxIdleConns(config.DBMaxIdleConns)
	database.SetConnMaxLifetime(time.Duration(config.DBConnMaxLifetime) * time.Second)
	defer database.Close()

	// Apply any required database migrations
	if err := db.RunMigrations(database); err != nil {
		slog.Error("Database migration failed", "err", err)
		os.Exit(1)
	}

	// Create the store
	store := db.NewStore(database)
	slog.Info("Database setup complete.")

	// Get the latest ledger sequence from the RPC server
	lastLedger, _, err := store.GetStatus(ctx, source)
	if err != nil {
		slog.Error("Failed to fetch last processed ledger", "err", err)
		os.Exit(1)
	}
	startSeq := max(lastLedger, config.LedgerBackendStartSeq)

	// Configure the RPC Ledger Backend
	var backend ledgerbackend.LedgerBackend
	if config.LedgerBackendType == "core" {
		var networkPassphrase string
		var defaultHistoryUrls []string
		if config.Network == "public" {
			networkPassphrase = network.PublicNetworkPassphrase
			defaultHistoryUrls = network.PublicNetworkhistoryArchiveURLs
		} else {
			networkPassphrase = network.TestNetworkPassphrase
			defaultHistoryUrls = network.TestNetworkhistoryArchiveURLs
		}
		defaultParams := ledgerbackend.CaptiveCoreTomlParams{
			NetworkPassphrase:  networkPassphrase,
			HistoryArchiveURLs: defaultHistoryUrls,
		}
		captiveCoreToml, err := ledgerbackend.NewCaptiveCoreTomlFromFile(config.CoreConfigPath, defaultParams)
		if err != nil {
			slog.Error("Failed to load captive core toml", "err", err)
			os.Exit(1)
		}
		captiveCoreConfig := ledgerbackend.CaptiveCoreConfig{
			BinaryPath:         config.CoreBinaryPath,
			NetworkPassphrase:  networkPassphrase,
			HistoryArchiveURLs: defaultHistoryUrls,
			Toml:               captiveCoreToml,
		}
		// Only log errors from the backend to keep output cleaner.
		lg := log.New()
		lg.SetLevel(logrus.WarnLevel)
		captiveCoreConfig.Log = lg
		backend, err = ledgerbackend.NewCaptive(captiveCoreConfig)
		if err != nil {
			slog.Error("Failed to create captive core backend", "err", err)
			os.Exit(1)
		}
		defer backend.Close()
	} else if config.LedgerBackendType == "rpc" {
		backend = ledgerbackend.NewRPCLedgerBackend(ledgerbackend.RPCLedgerBackendOptions{
			RPCServerURL: config.RPCUrl,
		})
		defer backend.Close()
	} else {
		slog.Error("Unsupported LEDGER_BACKEND_TYPE", "type", config.LedgerBackendType)
		os.Exit(1)
	}

	slog.Info("Setting up ledger ingestion service starting", "ledger", startSeq)
	if err := backend.PrepareRange(ctx, ledgerbackend.UnboundedRange(startSeq)); err != nil {
		slog.Error("Failed to prepare ledger range", "err", err)
		os.Exit(1)
	}
	slog.Info("Initial ledger range prepared.")

	idx := indexer.NewIndexer(store)

	slog.Info("Setup complete!")

	seq := startSeq
	for {
		ledger, err := backend.GetLedger(ctx, seq)
		if err != nil {
			slog.Error("No more ledgers or error at sequence.", "ledger", seq, "err", err)
			break
		}
		startTime := time.Now()

		txReader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(network.TestNetworkPassphrase, ledger)
		if err != nil {
			slog.Error("Failed to create transaction reader", "ledger", seq, "err", err)
			os.Exit(1)
		}

		idx.ApplyLedger(ctx, txReader, ledger.LedgerSequence(), ledger.LedgerCloseTime())

		err = store.UpsertStatus(ctx, source, ledger.LedgerSequence(), ledger.LedgerCloseTime())
		if err != nil {
			slog.Error("Failed to update last processed ledger", "ledger", seq, "err", err)
		}

		elapsed := time.Since(startTime)
		slog.Info("Ledger processed.", "ledger", ledger.LedgerSequence(), "ms", elapsed.Milliseconds())
		seq++
	}

	slog.Info("Indexer service stopped.")
}
