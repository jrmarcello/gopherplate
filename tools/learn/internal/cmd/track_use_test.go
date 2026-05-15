package cmd

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

// seedTrackStore opens a store with one skill and one memory row.
func seedTrackStore(t *testing.T) (string, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	st, openErr := store.Open(dbPath)
	if openErr != nil {
		t.Fatalf("store.Open: %v", openErr)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if upsertErr := st.UpsertSkillIndex(ctx, store.SkillIndexEntry{
		Path:            ".claude/skills/a/SKILL.md",
		Title:           "a",
		Body:            "body a",
		Tags:            "",
		FrontmatterJSON: "{}",
		IndexedAt:       now,
	}); upsertErr != nil {
		t.Fatalf("UpsertSkillIndex: %v", upsertErr)
	}
	if upsertErr := st.UpsertMemoryIndex(ctx, store.MemoryIndexEntry{
		Path:            "memory/b.md",
		Title:           "b",
		Body:            "body b",
		FrontmatterJSON: "{}",
		IndexedAt:       now,
	}); upsertErr != nil {
		t.Fatalf("UpsertMemoryIndex: %v", upsertErr)
	}
	return dbPath, st
}

// TC-UC-24 (REQ-17): track-use bumps usage_count for known skill and memory paths.
func TestRunTrackUse_happyPath(t *testing.T) {
	dbPath, st := seedTrackStore(t)
	var stderr bytes.Buffer
	const ts = "2026-05-14T12:00:00Z"
	parsed, _ := time.Parse(time.RFC3339, ts)

	runErr := runTrackUse(context.Background(), trackUseOpts{
		DBPath: dbPath,
		Paths:  []string{".claude/skills/a/SKILL.md", "memory/b.md"},
		Now:    func() time.Time { return parsed },
		Stderr: &stderr,
	})
	if runErr != nil {
		t.Fatalf("runTrackUse: %v", runErr)
	}

	// Verify skill_usage row.
	var (
		skillCount int
		skillTS    string
	)
	if scanErr := st.DB().QueryRow(
		`SELECT usage_count, last_used_at FROM skill_usage WHERE path = ?`,
		".claude/skills/a/SKILL.md",
	).Scan(&skillCount, &skillTS); scanErr != nil {
		t.Fatalf("query skill_usage: %v", scanErr)
	}
	if skillCount != 1 {
		t.Errorf("skill_usage.usage_count = %d, want 1", skillCount)
	}
	if skillTS != ts {
		t.Errorf("skill_usage.last_used_at = %q, want %q", skillTS, ts)
	}

	// Verify memory_usage row.
	var (
		memCount int
		memTS    string
	)
	if scanErr := st.DB().QueryRow(
		`SELECT usage_count, last_used_at FROM memory_usage WHERE path = ?`,
		"memory/b.md",
	).Scan(&memCount, &memTS); scanErr != nil {
		t.Fatalf("query memory_usage: %v", scanErr)
	}
	if memCount != 1 {
		t.Errorf("memory_usage.usage_count = %d, want 1", memCount)
	}
	if memTS != ts {
		t.Errorf("memory_usage.last_used_at = %q, want %q", memTS, ts)
	}

	// Re-run: counters should now be 2.
	if runErr := runTrackUse(context.Background(), trackUseOpts{
		DBPath: dbPath,
		Paths:  []string{".claude/skills/a/SKILL.md", "memory/b.md"},
		Now:    func() time.Time { return parsed },
		Stderr: &stderr,
	}); runErr != nil {
		t.Fatalf("runTrackUse #2: %v", runErr)
	}
	if scanErr := st.DB().QueryRow(
		`SELECT usage_count FROM skill_usage WHERE path = ?`,
		".claude/skills/a/SKILL.md",
	).Scan(&skillCount); scanErr != nil {
		t.Fatalf("re-query skill_usage: %v", scanErr)
	}
	if skillCount != 2 {
		t.Errorf("after 2 runs skill_usage.usage_count = %d, want 2", skillCount)
	}
}

// TC-UC-25 (REQ-17, infra): track-use on a path that exists in neither index
// is silent best-effort — exits 0, logs a warning.
func TestRunTrackUse_missingPathIsSilent(t *testing.T) {
	dbPath, st := seedTrackStore(t)
	var stderr bytes.Buffer

	runErr := runTrackUse(context.Background(), trackUseOpts{
		DBPath: dbPath,
		Paths:  []string{"this/does/not/exist.md"},
		Now:    time.Now,
		Stderr: &stderr,
	})
	if runErr != nil {
		t.Fatalf("runTrackUse: %v", runErr)
	}
	// Verify nothing was written.
	var count int
	if scanErr := st.DB().QueryRow(
		`SELECT COUNT(*) FROM skill_usage`,
	).Scan(&count); scanErr != nil {
		t.Fatalf("count skill_usage: %v", scanErr)
	}
	if count != 0 {
		t.Errorf("skill_usage rows = %d, want 0", count)
	}
	if scanErr := st.DB().QueryRow(
		`SELECT COUNT(*) FROM memory_usage`,
	).Scan(&count); scanErr != nil {
		t.Fatalf("count memory_usage: %v", scanErr)
	}
	if count != 0 {
		t.Errorf("memory_usage rows = %d, want 0", count)
	}
	// Verify the slog warning was emitted.
	if !strings.Contains(stderr.String(), "track-use missing path") {
		t.Errorf("expected warning 'track-use missing path' in stderr, got: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "this/does/not/exist.md") {
		t.Errorf("expected path in stderr, got: %q", stderr.String())
	}
}

// Empty --paths is a no-op success.
func TestRunTrackUse_emptyPathsNoOp(t *testing.T) {
	dbPath, _ := seedTrackStore(t)
	var stderr bytes.Buffer
	runErr := runTrackUse(context.Background(), trackUseOpts{
		DBPath: dbPath,
		Paths:  nil,
		Stderr: &stderr,
	})
	if runErr != nil {
		t.Fatalf("runTrackUse: %v", runErr)
	}
}
