package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(db *sql.DB) error {
	slog.Info("Applying database migrations...\n")

	// Create migrations tracking table
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version TEXT PRIMARY KEY,
            applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	// Sort migrations by filename
	var migrations []string
	for _, entry := range entries {
		if !entry.IsDir() {
			migrations = append(migrations, entry.Name())
		}
	}
	sort.Strings(migrations)

	// Apply each migration, skip if migration has already been applied
	for _, filename := range migrations {
		var exists bool
		err := db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
			filename,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}

		if exists {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		_, err = db.Exec(string(content))
		if err != nil {
			return fmt.Errorf("execute migration %s: %w", filename, err)
		}

		_, err = db.Exec(
			"INSERT INTO schema_migrations (version) VALUES ($1)",
			filename,
		)
		if err != nil {
			return fmt.Errorf("record migration %s: %w", filename, err)
		}

		slog.Info("Applied migration", "filename", filename)
	}

	slog.Info("Database migrations complete.")
	return nil
}
