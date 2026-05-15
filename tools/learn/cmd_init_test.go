package learn

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TC-UC-01: runInit on a fresh temp dir produces db.sqlite, config.yml, and a
// schema with all 10 base tables + 3 FTS5 virtual tables.
func TestRunInit_freshDirectory(t *testing.T) {
	dir := t.TempDir()
	learningDir := filepath.Join(dir, "learning")

	if runErr := runInit(initOpts{LearningDir: learningDir}); runErr != nil {
		t.Fatalf("runInit returned error: %v", runErr)
	}

	dbPath := filepath.Join(learningDir, "db.sqlite")
	if _, statErr := os.Stat(dbPath); statErr != nil {
		t.Fatalf("expected %s to exist: %v", dbPath, statErr)
	}
	configPath := filepath.Join(learningDir, "config.yml")
	if _, statErr := os.Stat(configPath); statErr != nil {
		t.Fatalf("expected %s to exist: %v", configPath, statErr)
	}

	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore after init: %v", openErr)
	}
	t.Cleanup(func() { _ = st.Close() })

	wantTables := []string{
		"events", "patterns", "candidates", "decisions",
		"skill_index", "memory_index", "skill_usage", "memory_usage",
		"nudge_state", "config_kv",
	}
	for _, table := range wantTables {
		var name string
		queryErr := st.DB().QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if queryErr != nil {
			t.Errorf("expected base table %q to exist: %v", table, queryErr)
		}
	}

	wantFTS := []string{"skill_fts", "memory_fts", "pattern_fts"}
	for _, table := range wantFTS {
		var name string
		queryErr := st.DB().QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if queryErr != nil {
			t.Errorf("expected FTS5 virtual table %q to exist: %v", table, queryErr)
		}
	}
}

// TC-UC-02: runInit is idempotent — running it twice does not destroy data
// inserted between the calls.
func TestRunInit_idempotentPreservesData(t *testing.T) {
	dir := t.TempDir()
	learningDir := filepath.Join(dir, "learning")

	if runErr := runInit(initOpts{LearningDir: learningDir}); runErr != nil {
		t.Fatalf("first runInit: %v", runErr)
	}

	dbPath := filepath.Join(learningDir, "db.sqlite")
	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}

	const specPath = "test.md"
	const sessionID = "sess-123"
	if _, execErr := st.DB().ExecContext(context.Background(),
		`INSERT INTO events (spec_path, session_id, completed_at) VALUES (?, ?, ?)`,
		specPath, sessionID, "2026-05-14T00:00:00Z"); execErr != nil {
		t.Fatalf("insert event: %v", execErr)
	}
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close store: %v", closeErr)
	}

	// Mutate config.yml to verify second init preserves user edits.
	configPath := filepath.Join(learningDir, "config.yml")
	const sentinel = "# user-edit-sentinel\n"
	original, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if writeErr := os.WriteFile(configPath, append([]byte(sentinel), original...), 0o644); writeErr != nil {
		t.Fatalf("write config: %v", writeErr)
	}

	if runErr := runInit(initOpts{LearningDir: learningDir}); runErr != nil {
		t.Fatalf("second runInit: %v", runErr)
	}

	// Event row still there.
	st2, openErr2 := openStore(dbPath)
	if openErr2 != nil {
		t.Fatalf("openStore after second init: %v", openErr2)
	}
	t.Cleanup(func() { _ = st2.Close() })

	var gotSpec, gotSession string
	queryErr := st2.DB().QueryRowContext(context.Background(),
		`SELECT spec_path, session_id FROM events WHERE session_id=?`, sessionID).Scan(&gotSpec, &gotSession)
	if queryErr != nil {
		t.Fatalf("event row missing after second init: %v", queryErr)
	}
	if gotSpec != specPath || gotSession != sessionID {
		t.Errorf("event corrupted: got (%q,%q), want (%q,%q)", gotSpec, gotSession, specPath, sessionID)
	}

	// Config preserved.
	final, readErr2 := os.ReadFile(configPath)
	if readErr2 != nil {
		t.Fatalf("read config after second init: %v", readErr2)
	}
	if string(final[:len(sentinel)]) != sentinel {
		t.Errorf("expected config.yml to be preserved (sentinel intact), got: %q", string(final))
	}
}

// TC-UC-32: runInit on a directory with no write permission returns a
// *runtimeError. Skipped on Windows and when running as root (chmod
// is effectively a no-op there).
func TestRunInit_unwritableParent_returnsRuntimeError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0555 semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 0555 is bypassed")
	}

	parent := t.TempDir()
	if chmodErr := os.Chmod(parent, 0o555); chmodErr != nil {
		t.Fatalf("chmod parent: %v", chmodErr)
	}
	// Restore to allow t.TempDir cleanup.
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	learningDir := filepath.Join(parent, "learning")
	runErr := runInit(initOpts{LearningDir: learningDir})
	if runErr == nil {
		t.Fatalf("expected runInit to fail on unwritable parent, got nil")
	}
	var re *runtimeError
	if !errors.As(runErr, &re) {
		t.Errorf("expected *runtimeError, got %T: %v", runErr, runErr)
	}
}
