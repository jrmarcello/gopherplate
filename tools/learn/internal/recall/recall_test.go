package recall

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/similar"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

// openTestStore opens an empty store under t.TempDir.
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	st, openErr := store.Open(dbPath)
	if openErr != nil {
		t.Fatalf("store.Open: %v", openErr)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// upsertSkill is a helper for seeding skill_index rows.
func upsertSkill(t *testing.T, st *store.Store, path, title, body string) {
	t.Helper()
	entry := store.SkillIndexEntry{
		Path:            path,
		Title:           title,
		Body:            body,
		Tags:            "",
		FrontmatterJSON: "{}",
		IndexedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	if upsertErr := st.UpsertSkillIndex(context.Background(), entry); upsertErr != nil {
		t.Fatalf("UpsertSkillIndex: %v", upsertErr)
	}
}

func upsertMemory(t *testing.T, st *store.Store, path, title, body string) {
	t.Helper()
	entry := store.MemoryIndexEntry{
		Path:            path,
		Title:           title,
		Body:            body,
		FrontmatterJSON: "{}",
		IndexedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	if upsertErr := st.UpsertMemoryIndex(context.Background(), entry); upsertErr != nil {
		t.Fatalf("UpsertMemoryIndex: %v", upsertErr)
	}
}

// TestRecall_emptyIndex returns no matches when the store has no rows.
func TestRecall_emptyIndex(t *testing.T) {
	st := openTestStore(t)
	matches, recErr := Recall(context.Background(), st.DB(), Options{
		Prompt:    "anything goes here",
		TopK:      3,
		MaxTokens: 500,
		MinScore:  0.0,
	})
	if recErr != nil {
		t.Fatalf("Recall: %v", recErr)
	}
	if len(matches) != 0 {
		t.Fatalf("len(matches) = %d, want 0", len(matches))
	}
}

// TestRecall_returnsTopK ranks results and caps to TopK.
func TestRecall_returnsTopK(t *testing.T) {
	st := openTestStore(t)
	upsertSkill(t, st, ".claude/skills/foo/SKILL.md", "foo", "the quick brown fox jumps")
	upsertSkill(t, st, ".claude/skills/bar/SKILL.md", "bar", "fox runs across the field")
	upsertSkill(t, st, ".claude/skills/baz/SKILL.md", "baz", "elephant in the room")

	matches, recErr := Recall(context.Background(), st.DB(), Options{
		Prompt:    "fox",
		TopK:      2,
		MaxTokens: 10000,
		MinScore:  0.0,
	})
	if recErr != nil {
		t.Fatalf("Recall: %v", recErr)
	}
	if len(matches) != 2 {
		t.Fatalf("len(matches) = %d, want 2", len(matches))
	}
	// Should not contain the elephant.
	for _, m := range matches {
		if strings.Contains(m.Path, "baz") {
			t.Errorf("unexpected baz match: %+v", m)
		}
	}
}

// TestRecall_minScoreFilter prunes matches below MinScore.
func TestRecall_minScoreFilter(t *testing.T) {
	st := openTestStore(t)
	upsertSkill(t, st, ".claude/skills/foo/SKILL.md", "foo", "fox fox fox fox")
	upsertSkill(t, st, ".claude/skills/bar/SKILL.md", "bar", "fox")

	// First, find what scores come back without filter.
	all, recErr := Recall(context.Background(), st.DB(), Options{
		Prompt:    "fox",
		TopK:      10,
		MaxTokens: 10000,
		MinScore:  0.0,
	})
	if recErr != nil {
		t.Fatalf("Recall(no-filter): %v", recErr)
	}
	if len(all) < 2 {
		t.Fatalf("expected 2 raw matches, got %d", len(all))
	}
	// Apply a min-score above the lowest result.
	midScore := all[len(all)-1].Score + 0.0001
	filtered, recallErr := Recall(context.Background(), st.DB(), Options{
		Prompt:    "fox",
		TopK:      10,
		MaxTokens: 10000,
		MinScore:  midScore,
	})
	if recallErr != nil {
		t.Fatalf("Recall(filter): %v", recallErr)
	}
	if len(filtered) >= len(all) {
		t.Errorf("filtered (%d) should be fewer than all (%d)", len(filtered), len(all))
	}
}

// TestRecall_maxTokensTruncation: when the cumulative summary length exceeds
// MaxTokens budget, the function drops the remainder (REQ-14 / TC-UC-22).
func TestRecall_maxTokensTruncation(t *testing.T) {
	st := openTestStore(t)
	// Seed 10 skill rows with long bodies (~200 chars each).
	long := strings.Repeat("fox ", 60) // ~240 chars
	for i := 0; i < 10; i++ {
		path := ".claude/skills/long" + string(rune('0'+i)) + "/SKILL.md"
		upsertSkill(t, st, path, "long"+string(rune('0'+i)), long)
	}

	// Each summary is ~120 chars. Budget 500 chars => ~4 summaries should fit.
	matches, recErr := Recall(context.Background(), st.DB(), Options{
		Prompt:    "fox",
		TopK:      10,
		MaxTokens: 500,
		MinScore:  0.0,
	})
	if recErr != nil {
		t.Fatalf("Recall: %v", recErr)
	}
	if len(matches) >= 10 {
		t.Errorf("expected truncation, got %d matches", len(matches))
	}
	if len(matches) == 0 {
		t.Fatalf("expected at least 1 match within budget")
	}
	// Every returned summary must be intact (no mid-truncation = its length must
	// be the same as it would be when computed directly: <=120 chars, ending
	// without ellipsis added by Recall itself).
	for _, m := range matches {
		if len(m.Summary) > summaryMaxChars {
			t.Errorf("summary length %d > summaryMaxChars %d (mid-truncation suspected)", len(m.Summary), summaryMaxChars)
		}
	}
	// Cumulative summary should fit within the budget (chars-as-tokens proxy).
	total := 0
	for _, m := range matches {
		total += len(m.Summary)
	}
	if total > 500 {
		t.Errorf("cumulative summary chars %d exceeds budget 500", total)
	}
}

// TestRecall_kindFilter restricts to memory_fts only when KindFilter=memory.
func TestRecall_kindFilter(t *testing.T) {
	st := openTestStore(t)
	upsertSkill(t, st, ".claude/skills/foo/SKILL.md", "foo skill", "fox jumps high")
	upsertMemory(t, st, "memory/foo.md", "foo memory", "fox flies low")

	matches, recErr := Recall(context.Background(), st.DB(), Options{
		Prompt:     "fox",
		TopK:       10,
		MaxTokens:  10000,
		MinScore:   0.0,
		KindFilter: "memory",
	})
	if recErr != nil {
		t.Fatalf("Recall: %v", recErr)
	}
	for _, m := range matches {
		if m.Kind != "memory" {
			t.Errorf("kind=%q, want memory", m.Kind)
		}
	}
	if len(matches) == 0 {
		t.Fatalf("expected at least one memory match")
	}
}

// TestRecall_topKBoundary: when TopK < raw match count, only TopK returned.
func TestRecall_topKBoundary(t *testing.T) {
	st := openTestStore(t)
	for i := 0; i < 5; i++ {
		path := ".claude/skills/foo" + string(rune('0'+i)) + "/SKILL.md"
		upsertSkill(t, st, path, "foo", "fox jumps")
	}
	matches, recErr := Recall(context.Background(), st.DB(), Options{
		Prompt:    "fox",
		TopK:      1,
		MaxTokens: 10000,
		MinScore:  0.0,
	})
	if recErr != nil {
		t.Fatalf("Recall: %v", recErr)
	}
	if len(matches) != 1 {
		t.Errorf("len(matches) = %d, want 1", len(matches))
	}
}

// Ensure compile-time: similar.Query is reachable.
var _ = similar.Query
var _ *sql.DB
