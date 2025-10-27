package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx"
	_ "modernc.org/sqlite"

	"github.com/script3/soroban-governor-backend/internal/api"
	"github.com/script3/soroban-governor-backend/internal/db"
)

func main() {
	ctx := context.Background()

	slog.Info("Starting API service...")

	slog.Info("Loading config...")
	config, err := api.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "err", err)
		os.Exit(1)
	}
	slog.Info("Config loaded.", "db_type", config.DBType, "port", config.APIPort)

	slog.Info("Connection to database...")
	// Create the database
	database, err := sql.Open(config.DBType, config.DBConnectionString)
	if err != nil {
		log.Fatal(err)
	}
	database.SetMaxOpenConns(config.DBMaxOpenConns)
	database.SetMaxIdleConns(config.DBMaxIdleConns)
	database.SetConnMaxLifetime(time.Duration(config.DBConnMaxLifetime) * time.Second)
	defer database.Close()

	// Create the store
	store := db.NewStore(database)
	slog.Info("Database connection complete.")

	// Create the API handler
	handler := api.NewHandler(store)

	// Setup HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", config.APIPort),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("Setup complete!")

	// Start server in a goroutine
	go func() {
		slog.Info("API server listening", "port", config.APIPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	slog.Info("API service stopped.")
}
