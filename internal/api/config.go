package api

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
	// API_PORT (string) default 8080
	// The port number for the API server to listen on.
	APIPort string
}

func LoadConfig() (*Config, error) {
	// Load .env file if it exists (for local development)
	// In production, this file won't exist and env vars will be injected by Docker
	if err := godotenv.Load("./config/api.cfg"); err != nil {
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

	// Load API_SERVER_PORT
	config.APIPort = os.Getenv("API_PORT")
	if config.APIPort == "" {
		slog.Info("API_PORT not set, defaulting to 8080")
		config.APIPort = "8080"
	}

	return config, nil
}
