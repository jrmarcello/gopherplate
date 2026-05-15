// SQLite persistence layer for the learning loop (REQ-18, REQ-19, REQ-19a).
//
// The store embeds schema.sql via go:embed and applies it on Open using
// CREATE ... IF NOT EXISTS so reopening an existing DB is a no-op. WAL mode,
// a 5 s busy timeout, and foreign-key enforcement are activated on every
// connection via pragmas issued at Open time.
package learn

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

//go:embed schema.sql
var schemaSQL string

// sqliteStore wraps a *sql.DB configured for the learning loop.
type sqliteStore struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path, configures pragmas
// (journal_mode=WAL, busy_timeout=5000, foreign_keys=ON), and applies the
// embedded schema idempotently. All failures are wrapped as
// *runtimeError so the dispatcher exits with code 2.
func openStore(path string) (*sqliteStore, error) {
	db, openErr := sql.Open("sqlite", path)
	if openErr != nil {
		return nil, runtimef("open store %q: %w", path, openErr)
	}

	// modernc.org/sqlite serializes writes through a single connection; cap the
	// pool conservatively so PRAGMA changes apply to the connections we hand out.
	db.SetMaxOpenConns(1)

	if pragmaErr := applyPragmas(db); pragmaErr != nil {
		_ = db.Close()
		return nil, runtimef("open store %q: %w", path, pragmaErr)
	}

	if migrationErr := applySchema(db); migrationErr != nil {
		_ = db.Close()
		return nil, runtimef("open store %q: %w", path, migrationErr)
	}

	// Tighten file permissions on the SQLite main file and its WAL/SHM
	// sidecars to 0600. The driver creates them with the umask default
	// (0644 on most Unix hosts), which leaves the DB world-readable on
	// multi-user systems — sanitization covers known secret classes but
	// any regex miss persists in plaintext, so defense-in-depth matters.
	// Errors are non-fatal: if chmod fails we still return the open sqliteStore
	// (it works); we just log via stderr-of-last-resort via the caller.
	if _, statErr := os.Stat(path); statErr == nil {
		_ = os.Chmod(path, 0o600)
	}
	_ = os.Chmod(path+"-wal", 0o600)
	_ = os.Chmod(path+"-shm", 0o600)

	return &sqliteStore{db: db}, nil
}

// Close releases the underlying database handle.
func (s *sqliteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB exposes the raw *sql.DB for ad-hoc queries until typed helpers cover
// every access pattern. Callers must respect the context-first convention.
func (s *sqliteStore) DB() *sql.DB { return s.db }

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

// storeEvent is one entry in the events table (REQ-18). Field names mirror columns
// in CamelCase Go form.
type storeEvent struct {
	SpecPath    string
	SessionID   string
	CompletedAt string
	CommitSHA   string
	TasksCount  int
	Outcome     string
}

// skillIndexEntry is one row in skill_index (REQ-19, REQ-19a). The companion
// skill_fts virtual table is kept in sync by AFTER INSERT/UPDATE/DELETE
// triggers defined in schema.sql.
type skillIndexEntry struct {
	Path            string
	Title           string
	Body            string
	Tags            string
	FrontmatterJSON string
	IndexedAt       string
}

// memoryIndexEntry mirrors skillIndexEntry for memory_index (no tags column).
type memoryIndexEntry struct {
	Path            string
	Title           string
	Body            string
	FrontmatterJSON string
	IndexedAt       string
}

// InsertEvent appends a row to events.
func (s *sqliteStore) InsertEvent(ctx context.Context, ev storeEvent) error {
	const q = `INSERT INTO events (spec_path, session_id, completed_at, commit_sha, tasks_count, outcome)
		VALUES (?, ?, ?, ?, ?, ?)`
	if _, execErr := s.db.ExecContext(ctx, q,
		ev.SpecPath, ev.SessionID, ev.CompletedAt, ev.CommitSHA, ev.TasksCount, ev.Outcome,
	); execErr != nil {
		return fmt.Errorf("insert event: %w", execErr)
	}
	return nil
}

// IncrementNudgeCounter atomically increments nudge_state.counter and returns
// the new value. The increment is a single SQL statement with RETURNING (SQLite
// >= 3.35) so concurrent callers cannot observe a race.
func (s *sqliteStore) IncrementNudgeCounter(ctx context.Context) (int, error) {
	const q = `UPDATE nudge_state SET counter = counter + 1 WHERE id = 1 RETURNING counter`
	var newCounter int
	if scanErr := s.db.QueryRowContext(ctx, q).Scan(&newCounter); scanErr != nil {
		return 0, fmt.Errorf("increment nudge counter: %w", scanErr)
	}
	return newCounter, nil
}

// GetNudgeCounter returns the current counter without mutating it.
func (s *sqliteStore) GetNudgeCounter(ctx context.Context) (int, error) {
	const q = `SELECT counter FROM nudge_state WHERE id = 1`
	var n int
	if scanErr := s.db.QueryRowContext(ctx, q).Scan(&n); scanErr != nil {
		return 0, fmt.Errorf("get nudge counter: %w", scanErr)
	}
	return n, nil
}

// ResetNudgeCounter sets counter back to 0 and records last_nudge_at as the
// current UTC timestamp.
func (s *sqliteStore) ResetNudgeCounter(ctx context.Context) error {
	const q = `UPDATE nudge_state SET counter = 0, last_nudge_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = 1`
	if _, execErr := s.db.ExecContext(ctx, q); execErr != nil {
		return fmt.Errorf("reset nudge counter: %w", execErr)
	}
	return nil
}

// MarkNudgeRun resets counter to 0 AND sets last_nudge_at to nowRFC3339 in a
// single atomic UPDATE. Idempotent: callable any number of times. Distinct
// from ResetNudgeCounter (which uses SQLite's strftime) because nudge-tick
// --reset accepts an injected `Now` for deterministic tests.
func (s *sqliteStore) MarkNudgeRun(ctx context.Context, nowRFC3339 string) error {
	const q = `UPDATE nudge_state SET counter = 0, last_nudge_at = ? WHERE id = 1`
	if _, execErr := s.db.ExecContext(ctx, q, nowRFC3339); execErr != nil {
		return fmt.Errorf("mark nudge run: %w", execErr)
	}
	return nil
}

// deprecationCandidate is one skill_index/memory_index row whose age and
// last-use stamps mark it for deprecation review (REQ-12).
type deprecationCandidate struct {
	Kind       string // "skill" or "memory"
	Path       string
	IndexedAt  string  // RFC3339
	LastUsedAt *string // nil when never used
	UsageCount int
}

// ListDeprecationCandidates returns rows from skill_index + skill_usage and
// memory_index + memory_usage that satisfy the REQ-12 deprecation criteria
// relative to nowRFC3339:
//
//   - indexed_at <= now - 2*ttl_days  (created proxy: indexed_at)
//   - AND (last_used_at IS NULL OR last_used_at <= now - ttl_days)
//
// Both bounds are inclusive (≤). Results are deterministic: sorted by
// (kind, path).
func (s *sqliteStore) ListDeprecationCandidates(ctx context.Context, nowRFC3339 string, ttlDays int) ([]deprecationCandidate, error) {
	// Compute thresholds in Go: avoids SQLite's date arithmetic quirks and
	// keeps the SQL trivially auditable.
	now, parseErr := time.Parse(time.RFC3339, nowRFC3339)
	if parseErr != nil {
		return nil, fmt.Errorf("list deprecation candidates: parse now %q: %w", nowRFC3339, parseErr)
	}
	now = now.UTC()
	createdThresh := now.AddDate(0, 0, -2*ttlDays).Format(time.RFC3339)
	usedThresh := now.AddDate(0, 0, -ttlDays).Format(time.RFC3339)

	const q = `
		SELECT 'skill' AS kind, si.path, si.indexed_at, su.last_used_at, COALESCE(su.usage_count, 0)
		FROM skill_index si
		LEFT JOIN skill_usage su ON su.path = si.path
		WHERE si.indexed_at <= ?
		  AND (su.last_used_at IS NULL OR su.last_used_at <= ?)
		UNION ALL
		SELECT 'memory' AS kind, mi.path, mi.indexed_at, mu.last_used_at, COALESCE(mu.usage_count, 0)
		FROM memory_index mi
		LEFT JOIN memory_usage mu ON mu.path = mi.path
		WHERE mi.indexed_at <= ?
		  AND (mu.last_used_at IS NULL OR mu.last_used_at <= ?)
		ORDER BY 1, 2`

	rows, queryErr := s.db.QueryContext(ctx, q, createdThresh, usedThresh, createdThresh, usedThresh)
	if queryErr != nil {
		return nil, fmt.Errorf("list deprecation candidates: %w", queryErr)
	}
	defer func() { _ = rows.Close() }()

	var out []deprecationCandidate
	for rows.Next() {
		var (
			c          deprecationCandidate
			lastUsedNS *string
		)
		if scanErr := rows.Scan(&c.Kind, &c.Path, &c.IndexedAt, &lastUsedNS, &c.UsageCount); scanErr != nil {
			return nil, fmt.Errorf("list deprecation candidates scan: %w", scanErr)
		}
		c.LastUsedAt = lastUsedNS
		out = append(out, c)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("list deprecation candidates iter: %w", rowsErr)
	}
	return out, nil
}

// UpsertSkillIndex inserts or replaces a row in skill_index. The AFTER triggers
// keep skill_fts in sync.
func (s *sqliteStore) UpsertSkillIndex(ctx context.Context, e skillIndexEntry) error {
	const q = `INSERT INTO skill_index (path, title, body, tags, frontmatter_json, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			body = excluded.body,
			tags = excluded.tags,
			frontmatter_json = excluded.frontmatter_json,
			indexed_at = excluded.indexed_at`
	if _, execErr := s.db.ExecContext(ctx, q,
		e.Path, e.Title, e.Body, e.Tags, e.FrontmatterJSON, e.IndexedAt,
	); execErr != nil {
		return fmt.Errorf("upsert skill_index %q: %w", e.Path, execErr)
	}
	return nil
}

// UpsertMemoryIndex mirrors UpsertSkillIndex for memory_index.
func (s *sqliteStore) UpsertMemoryIndex(ctx context.Context, e memoryIndexEntry) error {
	const q = `INSERT INTO memory_index (path, title, body, frontmatter_json, indexed_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			body = excluded.body,
			frontmatter_json = excluded.frontmatter_json,
			indexed_at = excluded.indexed_at`
	if _, execErr := s.db.ExecContext(ctx, q,
		e.Path, e.Title, e.Body, e.FrontmatterJSON, e.IndexedAt,
	); execErr != nil {
		return fmt.Errorf("upsert memory_index %q: %w", e.Path, execErr)
	}
	return nil
}

// DeleteOrphanSkill removes a skill_index row whose source file no longer
// exists on disk. The AFTER DELETE trigger purges the FTS row.
func (s *sqliteStore) DeleteOrphanSkill(ctx context.Context, path string) error {
	if _, execErr := s.db.ExecContext(ctx, `DELETE FROM skill_index WHERE path = ?`, path); execErr != nil {
		return fmt.Errorf("delete orphan skill %q: %w", path, execErr)
	}
	return nil
}

// DeleteOrphanMemory mirrors DeleteOrphanSkill for memory_index.
func (s *sqliteStore) DeleteOrphanMemory(ctx context.Context, path string) error {
	if _, execErr := s.db.ExecContext(ctx, `DELETE FROM memory_index WHERE path = ?`, path); execErr != nil {
		return fmt.Errorf("delete orphan memory %q: %w", path, execErr)
	}
	return nil
}

// IncrementSkillUsage bumps usage_count and last_used_at for path in
// skill_usage IFF path exists in skill_index (REQ-17). The single SQL
// statement is atomic; concurrent callers cannot observe a torn write.
//
// Returns (true, nil) when the skill exists and the row was upserted; returns
// (false, nil) when the path is not a known skill (caller should try
// IncrementMemoryUsage). Any DB failure surfaces as a wrapped error.
func (s *sqliteStore) IncrementSkillUsage(ctx context.Context, path, nowRFC3339 string) (bool, error) {
	const q = `INSERT INTO skill_usage(path, usage_count, last_used_at)
		SELECT ?, 1, ? FROM skill_index WHERE path = ?
		ON CONFLICT(path) DO UPDATE SET
			usage_count = usage_count + 1,
			last_used_at = excluded.last_used_at`
	res, execErr := s.db.ExecContext(ctx, q, path, nowRFC3339, path)
	if execErr != nil {
		return false, fmt.Errorf("increment skill_usage %q: %w", path, execErr)
	}
	rows, rowsErr := res.RowsAffected()
	if rowsErr != nil {
		return false, fmt.Errorf("increment skill_usage rows %q: %w", path, rowsErr)
	}
	return rows > 0, nil
}

// IncrementMemoryUsage mirrors IncrementSkillUsage for memory_index /
// memory_usage. See IncrementSkillUsage for the (bool, error) contract.
func (s *sqliteStore) IncrementMemoryUsage(ctx context.Context, path, nowRFC3339 string) (bool, error) {
	const q = `INSERT INTO memory_usage(path, usage_count, last_used_at)
		SELECT ?, 1, ? FROM memory_index WHERE path = ?
		ON CONFLICT(path) DO UPDATE SET
			usage_count = usage_count + 1,
			last_used_at = excluded.last_used_at`
	res, execErr := s.db.ExecContext(ctx, q, path, nowRFC3339, path)
	if execErr != nil {
		return false, fmt.Errorf("increment memory_usage %q: %w", path, execErr)
	}
	rows, rowsErr := res.RowsAffected()
	if rowsErr != nil {
		return false, fmt.Errorf("increment memory_usage rows %q: %w", path, rowsErr)
	}
	return rows > 0, nil
}
