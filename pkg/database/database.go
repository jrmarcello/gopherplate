package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// Config holds database connection configuration.
type Config struct {
	Driver          string // "postgres", "mysql", "sqlite3", etc.
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(driver, dsn string) Config {
	return Config{
		Driver:          driver,
		DSN:             dsn,
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 90 * time.Second,
	}
}

// DBCluster provides Writer/Reader split with automatic fallback.
// If no reader is configured, reader operations fall back to the writer.
type DBCluster struct {
	writer *sql.DB
	reader *sql.DB
}

// NewConnection creates a single database connection.
func NewConnection(cfg Config) (*sql.DB, error) {
	db, openErr := sql.Open(cfg.Driver, cfg.DSN)
	if openErr != nil {
		return nil, fmt.Errorf("failed to open database: %w", openErr)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if pingErr := db.PingContext(ctx); pingErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", pingErr)
	}

	return db, nil
}

// NewDBCluster creates a DBCluster with a writer and optional reader.
// If readerCfg is nil, reader operations will fall back to the writer.
func NewDBCluster(writerCfg Config, readerCfg *Config) (*DBCluster, error) {
	writer, writerErr := NewConnection(writerCfg)
	if writerErr != nil {
		return nil, fmt.Errorf("failed to connect writer: %w", writerErr)
	}

	cluster := &DBCluster{writer: writer}

	if readerCfg != nil && readerCfg.DSN != "" {
		reader, readerErr := NewConnection(*readerCfg)
		if readerErr != nil {
			// Log warning but don't fail — fall back to writer
			slog.Warn("Failed to connect reader, falling back to writer", "error", readerErr)
		} else {
			cluster.reader = reader
		}
	}

	return cluster, nil
}

// NewDBClusterFromDB creates a DBCluster from an existing *sql.DB connection.
// The same connection is used for both writer and reader.
// Useful for tests where a single connection is sufficient.
func NewDBClusterFromDB(db *sql.DB) *DBCluster {
	return &DBCluster{writer: db}
}

// Writer returns the writer database connection.
func (c *DBCluster) Writer() *sql.DB {
	return c.writer
}

// Reader returns the reader database connection.
// Falls back to writer if no reader is configured.
func (c *DBCluster) Reader() *sql.DB {
	if c.reader != nil {
		return c.reader
	}
	return c.writer
}

// HasSeparateReader returns true if a separate reader is configured.
func (c *DBCluster) HasSeparateReader() bool {
	return c.reader != nil
}

// PingAll pings all database connections for health checks.
func (c *DBCluster) PingAll(ctx context.Context) error {
	if pingErr := c.writer.PingContext(ctx); pingErr != nil {
		return fmt.Errorf("writer ping failed: %w", pingErr)
	}
	if c.reader != nil {
		if pingErr := c.reader.PingContext(ctx); pingErr != nil {
			return fmt.Errorf("reader ping failed: %w", pingErr)
		}
	}
	return nil
}

// Close closes all database connections.
func (c *DBCluster) Close() error {
	var closeErr error
	if writerErr := c.writer.Close(); writerErr != nil {
		closeErr = fmt.Errorf("failed to close writer: %w", writerErr)
	}
	if c.reader != nil {
		if readerErr := c.reader.Close(); readerErr != nil {
			if closeErr != nil {
				closeErr = fmt.Errorf("%w; failed to close reader: %w", closeErr, readerErr)
			} else {
				closeErr = fmt.Errorf("failed to close reader: %w", readerErr)
			}
		}
	}
	return closeErr
}
