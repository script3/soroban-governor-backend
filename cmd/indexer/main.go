package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"os"

	"github.com/script3/soroban-governor-backend/internal/db"
	"github.com/script3/soroban-governor-backend/internal/indexer"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/network"

	_ "github.com/jackc/pgx"
	_ "modernc.org/sqlite"
)

func main() {
	ctx := context.Background()
	source := "indexer"

	slog.Info("Starting indexer service...")

	slog.Info("Setting up database...")
	// Create the database
	database, err := sql.Open("sqlite", "file:./gov.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// Apply any required database migrations
	if err := db.RunMigrations(database); err != nil {
		slog.Error("Database migration failed", "err", err)
		os.Exit(1)
	}

	// Create the store
	store := db.NewStore(database)
	slog.Info("Database setup complete.")

	// Use the public SDF Testnet RPC for demo purpose
	endpoint := "https://soroban-testnet.stellar.org"

	// Get the latest ledger sequence from the RPC server
	lastLedger, err := store.GetLedgerSeq(ctx, source)
	if err != nil {
		slog.Error("Failed to fetch last processed ledger", "err", err)
		os.Exit(1)
	}

	startSeq := max(lastLedger, 1209657)

	// Fetch the most recently processed ledger, if any.

	// Configure the RPC Ledger Backend
	backend := ledgerbackend.NewRPCLedgerBackend(ledgerbackend.RPCLedgerBackendOptions{
		RPCServerURL: endpoint,
	})
	defer backend.Close()

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

		txReader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(network.TestNetworkPassphrase, ledger)
		if err != nil {
			slog.Error("Failed to create transaction reader", "ledger", seq, "err", err)
			os.Exit(1)
		}

		idx.ApplyLedger(ctx, txReader, ledger.LedgerSequence(), ledger.LedgerCloseTime())

		err = store.UpsertLedgerSeq(ctx, source, ledger.LedgerSequence())
		if err != nil {
			slog.Error("Failed to update last processed ledger", "ledger", seq, "err", err)
		}

		slog.Info("Ledger processed.", "ledger", ledger.LedgerSequence())
		seq++
	}

	slog.Info("Indexer service stopped.")
}
