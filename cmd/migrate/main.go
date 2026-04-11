// Package main provides the migration binary for database schema management.
//
// This follows the idiomatic Go pattern of separate binaries in cmd/:
//   - cmd/api/     → HTTP server
//   - cmd/migrate/ → Database migrations
//
// Usage:
//
//	./migrate           # Apply all pending migrations
//	./migrate --version # Show goose version
//
// The binary is designed to be run as a Kubernetes Job with ArgoCD PreSync hook,
// ensuring migrations complete successfully before the application deployment.
package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"

	"github.com/jrmarcello/go-boilerplate/config"
)

func main() {
	if runErr := run(); runErr != nil {
		slog.Error("migration failed", "error", runErr)
		os.Exit(1)
	}
}

func run() error {
	slog.Info("starting database migrations")

	// Load configuration
	cfg, loadErr := config.Load()
	if loadErr != nil {
		return fmt.Errorf("loading config: %w", loadErr)
	}

	// Connect to database (always use writer for migrations)
	db, openErr := sql.Open("postgres", cfg.DB.GetWriterDSN())
	if openErr != nil {
		return fmt.Errorf("opening database connection: %w", openErr)
	}
	defer func() { _ = db.Close() }()

	// Verify connection
	if pingErr := db.Ping(); pingErr != nil {
		return fmt.Errorf("pinging database: %w", pingErr)
	}
	slog.Info("database connection established")

	// Run migrations
	migrationsDir := getMigrationsDir()
	slog.Info("running migrations", "dir", migrationsDir)

	if dialectErr := goose.SetDialect("postgres"); dialectErr != nil {
		return fmt.Errorf("setting goose dialect: %w", dialectErr)
	}

	if upErr := goose.Up(db, migrationsDir); upErr != nil {
		return fmt.Errorf("running migrations: %w", upErr)
	}

	slog.Info("migrations completed successfully")
	return nil
}

// getMigrationsDir returns the path to migrations directory.
// In production (Docker), migrations are at ./migrations.
// In development, they're at internal/infrastructure/db/postgres/migration.
func getMigrationsDir() string {
	// Production: migrations are copied to ./migrations in Dockerfile
	if _, statErr := os.Stat("./migrations"); statErr == nil {
		return "./migrations"
	}

	// Development: use source path
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "./migrations"
	}
	cmdDir := filepath.Dir(currentFile)
	return filepath.Join(cmdDir, "..", "..", "internal", "infrastructure", "db", "postgres", "migration")
}
