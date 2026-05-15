// Package store provides the SQLite persistence layer for the learning loop
// (REQ-18, REQ-19, REQ-19a).
//
// The store embeds schema.sql via go:embed and applies it on Open using
// CREATE ... IF NOT EXISTS so reopening an existing DB is a no-op. WAL mode,
// a 5 s busy timeout, and foreign-key enforcement are activated on every
// connection via pragmas issued at Open time.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

//go:embed schema.sql
var schemaSQL string

// Store wraps a *sql.DB configured for the learning loop.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path, configures pragmas
// (journal_mode=WAL, busy_timeout=5000, foreign_keys=ON), and applies the
// embedded schema idempotently. All failures are wrapped as
// *learnerr.RuntimeError so the dispatcher exits with code 2.
func Open(path string) (*Store, error) {
	db, openErr := sql.Open("sqlite", path)
	if openErr != nil {
		return nil, learnerr.Runtimef("open store %q: %w", path, openErr)
	}

	// modernc.org/sqlite serializes writes through a single connection; cap the
	// pool conservatively so PRAGMA changes apply to the connections we hand out.
	db.SetMaxOpenConns(1)

	if pragmaErr := applyPragmas(db); pragmaErr != nil {
		_ = db.Close()
		return nil, learnerr.Runtimef("open store %q: %w", path, pragmaErr)
	}

	if migrationErr := applySchema(db); migrationErr != nil {
		_ = db.Close()
		return nil, learnerr.Runtimef("open store %q: %w", path, migrationErr)
	}

	// Tighten file permissions on the SQLite main file and its WAL/SHM
	// sidecars to 0600. The driver creates them with the umask default
	// (0644 on most Unix hosts), which leaves the DB world-readable on
	// multi-user systems — sanitization covers known secret classes but
	// any regex miss persists in plaintext, so defense-in-depth matters.
	// Errors are non-fatal: if chmod fails we still return the open Store
	// (it works); we just log via stderr-of-last-resort via the caller.
	if _, statErr := os.Stat(path); statErr == nil {
		_ = os.Chmod(path, 0o600)
	}
	_ = os.Chmod(path+"-wal", 0o600)
	_ = os.Chmod(path+"-shm", 0o600)

	return &Store{db: db}, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB exposes the raw *sql.DB for ad-hoc queries until typed helpers cover
// every access pattern. Callers must respect the context-first convention.
func (s *Store) DB() *sql.DB { return s.db }

// applyPragmas configures connection-level settings. journal_mode is verified
// to ensure the file ended up in WAL mode (mandatory for concurrent reads).
func applyPragmas(db *sql.DB) error {
	var mode string
	if scanErr := db.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); scanErr != nil {
		return fmt.Errorf("pragma journal_mode: %w", scanErr)
	}
	if mode != "wal" {
		return fmt.Errorf("pragma journal_mode: got %q, want wal", mode)
	}

	if _, execErr := db.Exec(`PRAGMA busy_timeout=5000`); execErr != nil {
		return fmt.Errorf("pragma busy_timeout: %w", execErr)
	}
	if _, execErr := db.Exec(`PRAGMA foreign_keys=ON`); execErr != nil {
		return fmt.Errorf("pragma foreign_keys: %w", execErr)
	}
	return nil
}

// applySchema executes the embedded schema.sql. Every DDL uses IF NOT EXISTS
// so calling this on an already-migrated DB is safe.
func applySchema(db *sql.DB) error {
	if _, execErr := db.Exec(schemaSQL); execErr != nil {
		return fmt.Errorf("apply schema: %w", execErr)
	}
	return nil
}
