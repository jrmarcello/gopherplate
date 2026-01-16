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
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"

	"bitbucket.org/appmax-space/go-boilerplate/config"
)

func main() {
	slog.Info("starting database migrations")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Connect to database
	db, err := sql.Open("postgres", cfg.DB.DSN)
	if err != nil {
		slog.Error("failed to open database connection", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("database connection established")

	// Run migrations
	migrationsDir := getMigrationsDir()
	slog.Info("running migrations", "dir", migrationsDir)

	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("failed to set goose dialect", "error", err)
		os.Exit(1)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		slog.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	slog.Info("migrations completed successfully")
}

// getMigrationsDir returns the path to migrations directory.
// In production (Docker), migrations are at ./migrations.
// In development, they're at internal/infrastructure/db/postgres/migration.
func getMigrationsDir() string {
	// Production: migrations are copied to ./migrations in Dockerfile
	if _, err := os.Stat("./migrations"); err == nil {
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
