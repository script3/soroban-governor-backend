package indexer

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// DB_TYPE (string) default "sqlite"
	// The type of database to use. Supported values are "sqlite" and "postgres".
	DBType string
	// DB_CONNECTION_STRING (string) default ":memory:"
	// Sets the database connection string.
	// - For sqlite, this is often file path like "file:./gov.db". In memory is ":memory:".
	// - For postgres, this is a full connection string like "postgres://user:pass@localhost:5432/dbname"
	DBConnectionString string
	// DB_INDEXER_MAX_OPEN_CONNS (int) default 30
	// The maximum number of open connections to the database for the indexer. The indexer processes events
	// in order, so the not many concurrent connections are needed.
	DBMaxOpenConns int
	// DB_INDEXER_MAX_IDLE_CONNS (int) default 10
	// The maximum number of idle connections to the database for the indexer.
	DBMaxIdleConns int
	// DB_INDEXER_CONN_MAX_LIFETIME (int) default 300
	// The maximum lifetime (in seconds) of a database connection for the indexer.
	DBConnMaxLifetime int

	// NETWORK (string) default "testnet"
	// The Stellar network to connect to. Supported values are "public", "testnet", and "standalone".
	Network string

	// LEDGER_BACKEND_TYPE (string) default "rpc"
	// The type of ledger source to use for the indexer. Supported values are "rpc" and "core".
	// Core will use a captive core instance, and will expect a core config file to be present.
	// If using captive core, it is recommended to also persist the core database to the same volume
	LedgerBackendType string

	// LEDGER_BACKEND_START_SEQ (int) default 10
	// The ledger sequence number to start indexing from, if no previous state is found in the database.
	// This must be greater than the genesis ledger of the network being indexed. For the public network, it's
	// recommended to use at least the ledger where Soroban was enabled (50457424)
	LedgerBackendStartSeq uint32

	// RPC_URL (string) default "https://soroban-testnet.stellar.org"
	// The URL of the Stellar RPC server to connect to, if using "rpc" as the ledger backend.
	RPCUrl string

	// CORE_CONFIG_PATH (string) default "/config/stellar-core.cfg"
	// The file path to the stellar-core config file, if using "core" as the ledger backend.
	// CORE_CONFIG_PATH=/mount/stellar-core.cfg
	CoreConfigPath string

	// CORE_BINARY_PATH (string) default "/usr/bin/stellar-core"
	// The file path to the stellar-core binary, if using "core" as the ledger backend.
	CoreBinaryPath string
}

func LoadConfig() (*Config, error) {
	// Load .env file if it exists (for local development)
	// In production, this file won't exist and env vars will be injected by Docker
	if err := godotenv.Load("./config/indexer.cfg"); err != nil {
		slog.Info("No config file found. Loading configuration from environment variables only.")
	}

	config := &Config{}

	// Load DB_TYPE
	config.DBType = os.Getenv("DB_TYPE")
	if config.DBType == "" {
		slog.Info("DB_TYPE not set, defaulting to sqlite")
		config.DBType = "sqlite"
	}

	// Load DB_CONNECTION_STRING
	config.DBConnectionString = os.Getenv("DB_CONNECTION_STRING")
	if config.DBConnectionString == "" {
		slog.Info("DB_CONNECTION_STRING not set, defaulting to in-memory database")
		config.DBConnectionString = ":memory:"
	}

	// Load DB_INDEXER_MAX_OPEN_CONNS
	config.DBMaxOpenConns = 30
	val := os.Getenv("DB_MAX_OPEN_CONNS")
	if val != "" {
		var err error
		config.DBMaxOpenConns, err = strconv.Atoi(val)
		if err != nil {
			return nil, err
		}
	} else {
		slog.Info("DB_MAX_OPEN_CONNS not set, defaulting to 30")
	}

	// Load DB_INDEXER_MAX_IDLE_CONNS
	config.DBMaxIdleConns = 10
	val = os.Getenv("DB_MAX_IDLE_CONNS")
	if val != "" {
		var err error
		config.DBMaxIdleConns, err = strconv.Atoi(val)
		if err != nil {
			return nil, err
		}
	} else {
		slog.Info("DB_MAX_IDLE_CONNS not set, defaulting to 10")
	}

	// Load DB_INDEXER_CONN_MAX_LIFETIME
	config.DBConnMaxLifetime = 300
	val = os.Getenv("DB_CONN_MAX_LIFETIME")
	if val != "" {
		var err error
		config.DBConnMaxLifetime, err = strconv.Atoi(val)
		if err != nil {
			return nil, err
		}
	} else {
		slog.Info("DB_CONN_MAX_LIFETIME not set, defaulting to 300")
	}

	// Load NETWORK
	config.Network = os.Getenv("NETWORK")
	if config.Network == "" {
		slog.Info("NETWORK not set, defaulting to testnet")
		config.Network = "testnet"
	}

	// Load LEDGER_BACKEND_TYPE
	config.LedgerBackendType = os.Getenv("LEDGER_BACKEND_TYPE")
	if config.LedgerBackendType == "" {
		slog.Info("LEDGER_BACKEND_TYPE not set, defaulting to rpc")
		config.LedgerBackendType = "rpc"
	}

	// Load LEDGER_BACKEND_START_SEQ
	config.LedgerBackendStartSeq = 10
	val = os.Getenv("LEDGER_BACKEND_START_SEQ")
	if val != "" {
		seq, err := strconv.ParseUint(val, 10, 32)
		if err != nil {
			return nil, err
		}
		config.LedgerBackendStartSeq = uint32(seq)
	} else {
		slog.Info("LEDGER_BACKEND_START_SEQ not set, defaulting to 10")
	}

	// Load RPC_URL
	config.RPCUrl = os.Getenv("RPC_URL")
	if config.RPCUrl == "" {
		slog.Info("RPC_URL not set, defaulting to https://soroban-testnet.stellar.org")
		config.RPCUrl = "https://soroban-testnet.stellar.org"
	}

	// Load CORE_CONFIG_PATH
	config.CoreConfigPath = os.Getenv("CORE_CONFIG_PATH")
	if config.CoreConfigPath == "" {
		slog.Info("CORE_CONFIG_PATH not set, defaulting to /config/stellar-core.cfg")
		config.CoreConfigPath = "/config/stellar-core.cfg"
	}

	// Load CORE_BINARY_PATH
	config.CoreBinaryPath = os.Getenv("CORE_BINARY_PATH")
	if config.CoreBinaryPath == "" {
		slog.Info("CORE_BINARY_PATH not set, defaulting to /usr/bin/stellar-core")
		config.CoreBinaryPath = "/usr/bin/stellar-core"
	}

	return config, nil
}
