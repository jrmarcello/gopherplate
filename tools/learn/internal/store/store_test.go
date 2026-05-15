package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
)

// TC-UC-01 (parte): Open on empty path creates DB, expected tables exist,
// journal_mode is "wal", busy_timeout is 5000.
func TestOpen_CreatesSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "learn.db")

	s, openErr := Open(dbPath)
	if openErr != nil {
		t.Fatalf("Open returned error: %v", openErr)
	}
	t.Cleanup(func() { _ = s.Close() })

	expectedTables := []string{
		"events", "patterns", "candidates", "decisions",
		"skill_index", "memory_index", "skill_usage", "memory_usage",
		"nudge_state", "config_kv",
	}
	for _, name := range expectedTables {
		var got string
		queryErr := s.DB().QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&got)
		if queryErr != nil {
			t.Errorf("table %q missing: %v", name, queryErr)
		}
	}

	var mode string
	if pragmaErr := s.DB().QueryRowContext(context.Background(),
		`PRAGMA journal_mode`).Scan(&mode); pragmaErr != nil {
		t.Fatalf("PRAGMA journal_mode failed: %v", pragmaErr)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}

	var timeout int
	if pragmaErr := s.DB().QueryRowContext(context.Background(),
		`PRAGMA busy_timeout`).Scan(&timeout); pragmaErr != nil {
		t.Fatalf("PRAGMA busy_timeout failed: %v", pragmaErr)
	}
	if timeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", timeout)
	}
}

// TC-UC-01a (REQ-18): FTS5 virtual tables exist after Open.
func TestOpen_CreatesFTSVirtualTables(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "learn.db")
	s, openErr := Open(dbPath)
	if openErr != nil {
		t.Fatalf("Open: %v", openErr)
	}
	t.Cleanup(func() { _ = s.Close() })

	expected := []string{"skill_fts", "memory_fts", "pattern_fts"}
	for _, name := range expected {
		var got string
		queryErr := s.DB().QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&got)
		if queryErr != nil {
			t.Errorf("virtual table %q missing: %v", name, queryErr)
		}
	}
}

// TC-UC-02 (REQ-18): Open is idempotent — re-open preserves data.
func TestOpen_Idempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "learn.db")
	ctx := context.Background()

	s1, openErr := Open(dbPath)
	if openErr != nil {
		t.Fatalf("Open #1: %v", openErr)
	}
	insertErr := s1.InsertEvent(ctx, Event{
		SpecPath:    ".specs/foo.md",
		SessionID:   "sess-1",
		CompletedAt: "2026-01-01T00:00:00Z",
		CommitSHA:   "abc123",
		TasksCount:  3,
		Outcome:     "success",
	})
	if insertErr != nil {
		t.Fatalf("InsertEvent: %v", insertErr)
	}
	if closeErr := s1.Close(); closeErr != nil {
		t.Fatalf("Close #1: %v", closeErr)
	}

	s2, reopenErr := Open(dbPath)
	if reopenErr != nil {
		t.Fatalf("Open #2: %v", reopenErr)
	}
	t.Cleanup(func() { _ = s2.Close() })

	var count int
	scanErr := s2.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&count)
	if scanErr != nil {
		t.Fatalf("count: %v", scanErr)
	}
	if count != 1 {
		t.Errorf("event count after reopen = %d, want 1", count)
	}
}

// TC-UC-31 (REQ-21): corrupted DB file -> Open returns *learnerr.RuntimeError.
func TestOpen_CorruptedDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "corrupt.db")
	// Write garbage bytes that do not form a valid SQLite header.
	if writeErr := os.WriteFile(dbPath, []byte("not a sqlite database at all, just garbage\x00\xff\xfe"), 0o600); writeErr != nil {
		t.Fatalf("write garbage: %v", writeErr)
	}

	_, openErr := Open(dbPath)
	if openErr == nil {
		t.Fatalf("expected error on corrupted DB")
	}
	var re *learnerr.RuntimeError
	if !errors.As(openErr, &re) {
		t.Errorf("expected *learnerr.RuntimeError, got %T: %v", openErr, openErr)
	}
}

// IncrementNudgeCounter atomicity: N concurrent increments produce exactly N.
// t.Parallel: this concurrency test has its own DB and benefits from running
// alongside sibling store tests so the -race detector surfaces races caused
// by global state at module init time (none expected, but defense-in-depth).
func TestIncrementNudgeCounter_Atomic(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "learn.db")
	s, openErr := Open(dbPath)
	if openErr != nil {
		t.Fatalf("Open: %v", openErr)
	}
	t.Cleanup(func() { _ = s.Close() })

	const N = 10
	var wg sync.WaitGroup
	errCh := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, incErr := s.IncrementNudgeCounter(context.Background()); incErr != nil {
				errCh <- incErr
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		t.Errorf("increment error: %v", e)
	}

	final, getErr := s.GetNudgeCounter(context.Background())
	if getErr != nil {
		t.Fatalf("GetNudgeCounter: %v", getErr)
	}
	if final != N {
		t.Errorf("counter = %d, want %d", final, N)
	}
}

// ResetNudgeCounter sets counter back to 0.
func TestResetNudgeCounter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "learn.db")
	s, openErr := Open(dbPath)
	if openErr != nil {
		t.Fatalf("Open: %v", openErr)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, incErr := s.IncrementNudgeCounter(ctx); incErr != nil {
			t.Fatalf("inc: %v", incErr)
		}
	}
	if resetErr := s.ResetNudgeCounter(ctx); resetErr != nil {
		t.Fatalf("reset: %v", resetErr)
	}
	got, getErr := s.GetNudgeCounter(ctx)
	if getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if got != 0 {
		t.Errorf("counter after reset = %d, want 0", got)
	}
}

// FTS5 sync: insert into skill_index -> MATCH query on skill_fts finds it.
func TestUpsertSkillIndex_FTSSync(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "learn.db")
	s, openErr := Open(dbPath)
	if openErr != nil {
		t.Fatalf("Open: %v", openErr)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	entry := SkillIndexEntry{
		Path:            ".claude/skills/foo/SKILL.md",
		Title:           "Foo Skill",
		Body:            "this skill handles foobar processing",
		Tags:            "tag1,tag2",
		FrontmatterJSON: `{"name":"foo"}`,
		IndexedAt:       "2026-01-01T00:00:00Z",
	}
	if upsertErr := s.UpsertSkillIndex(ctx, entry); upsertErr != nil {
		t.Fatalf("UpsertSkillIndex: %v", upsertErr)
	}

	var path string
	queryErr := s.DB().QueryRowContext(ctx,
		`SELECT path FROM skill_fts WHERE skill_fts MATCH ?`, "foobar").Scan(&path)
	if queryErr != nil {
		t.Fatalf("FTS query: %v", queryErr)
	}
	if path != entry.Path {
		t.Errorf("FTS match path = %q, want %q", path, entry.Path)
	}

	// Upsert again to verify UPDATE branch syncs FTS too.
	entry.Body = "updated body mentioning bazqux"
	if upsertErr := s.UpsertSkillIndex(ctx, entry); upsertErr != nil {
		t.Fatalf("UpsertSkillIndex update: %v", upsertErr)
	}
	var got string
	if queryErr := s.DB().QueryRowContext(ctx,
		`SELECT body FROM skill_fts WHERE skill_fts MATCH ?`, "bazqux").Scan(&got); queryErr != nil {
		t.Fatalf("FTS re-query: %v", queryErr)
	}
}

// Memory index FTS sync (mirror of skill test, smaller).
func TestUpsertMemoryIndex_FTSSync(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "learn.db")
	s, openErr := Open(dbPath)
	if openErr != nil {
		t.Fatalf("Open: %v", openErr)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	entry := MemoryIndexEntry{
		Path:            "memory/feedback_quality.md",
		Title:           "Quality First",
		Body:            "prefer rigor over speed in implementation",
		FrontmatterJSON: `{}`,
		IndexedAt:       "2026-01-01T00:00:00Z",
	}
	if upsertErr := s.UpsertMemoryIndex(ctx, entry); upsertErr != nil {
		t.Fatalf("UpsertMemoryIndex: %v", upsertErr)
	}
	var path string
	if queryErr := s.DB().QueryRowContext(ctx,
		`SELECT path FROM memory_fts WHERE memory_fts MATCH ?`, "rigor").Scan(&path); queryErr != nil {
		t.Fatalf("FTS query: %v", queryErr)
	}
	if path != entry.Path {
		t.Errorf("FTS match path = %q, want %q", path, entry.Path)
	}
}

// DeleteOrphan removes row from base table and FTS.
func TestDeleteOrphanSkill(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "learn.db")
	s, openErr := Open(dbPath)
	if openErr != nil {
		t.Fatalf("Open: %v", openErr)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	entry := SkillIndexEntry{
		Path:            ".claude/skills/bar/SKILL.md",
		Title:           "Bar",
		Body:            "uniqueterm bar",
		Tags:            "",
		FrontmatterJSON: `{}`,
		IndexedAt:       "2026-01-01T00:00:00Z",
	}
	if upsertErr := s.UpsertSkillIndex(ctx, entry); upsertErr != nil {
		t.Fatalf("upsert: %v", upsertErr)
	}
	if delErr := s.DeleteOrphanSkill(ctx, entry.Path); delErr != nil {
		t.Fatalf("delete: %v", delErr)
	}
	var count int
	if scanErr := s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM skill_index WHERE path=?`, entry.Path).Scan(&count); scanErr != nil {
		t.Fatalf("count base: %v", scanErr)
	}
	if count != 0 {
		t.Errorf("skill_index count = %d, want 0", count)
	}
	if scanErr := s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM skill_fts WHERE skill_fts MATCH ?`, "uniqueterm").Scan(&count); scanErr != nil {
		t.Fatalf("count fts: %v", scanErr)
	}
	if count != 0 {
		t.Errorf("skill_fts count = %d, want 0", count)
	}
}
