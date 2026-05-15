package learn

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// openTestStoreForRecall opens an empty store under t.TempDir for recall tests.
func openTestStoreForRecall(t *testing.T) *sqliteStore {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("Open: %v", openErr)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func upsertSkillForRecall(t *testing.T, st *sqliteStore, path, title, body string) {
	t.Helper()
	entry := skillIndexEntry{
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

func upsertMemoryForRecall(t *testing.T, st *sqliteStore, path, title, body string) {
	t.Helper()
	entry := memoryIndexEntry{
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
	st := openTestStoreForRecall(t)
	matches, recErr := recallMatches(context.Background(), st.DB(), recallOptions{
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
	st := openTestStoreForRecall(t)
	upsertSkillForRecall(t, st, ".claude/skills/foo/SKILL.md", "foo", "the quick brown fox jumps")
	upsertSkillForRecall(t, st, ".claude/skills/bar/SKILL.md", "bar", "fox runs across the field")
	upsertSkillForRecall(t, st, ".claude/skills/baz/SKILL.md", "baz", "elephant in the room")

	matches, recErr := recallMatches(context.Background(), st.DB(), recallOptions{
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
	for _, m := range matches {
		if strings.Contains(m.Path, "baz") {
			t.Errorf("unexpected baz match: %+v", m)
		}
	}
}

// TestRecall_minScoreFilter prunes matches below MinScore.
func TestRecall_minScoreFilter(t *testing.T) {
	st := openTestStoreForRecall(t)
	upsertSkillForRecall(t, st, ".claude/skills/foo/SKILL.md", "foo", "fox fox fox fox")
	upsertSkillForRecall(t, st, ".claude/skills/bar/SKILL.md", "bar", "fox")

	all, recErr := recallMatches(context.Background(), st.DB(), recallOptions{
		Prompt:    "fox",
		TopK:      10,
		MaxTokens: 10000,
		MinScore:  0.0,
	})
	if recErr != nil {
		t.Fatalf("recallMatches(no-filter): %v", recErr)
	}
	if len(all) < 2 {
		t.Fatalf("expected 2 raw matches, got %d", len(all))
	}
	midScore := all[len(all)-1].Score + 0.0001
	filtered, recallErr := recallMatches(context.Background(), st.DB(), recallOptions{
		Prompt:    "fox",
		TopK:      10,
		MaxTokens: 10000,
		MinScore:  midScore,
	})
	if recallErr != nil {
		t.Fatalf("recallMatches(filter): %v", recallErr)
	}
	if len(filtered) >= len(all) {
		t.Errorf("filtered (%d) should be fewer than all (%d)", len(filtered), len(all))
	}
}

// TestRecall_maxTokensTruncation: when the cumulative summary length exceeds
// MaxTokens budget, the function drops the remainder (REQ-14 / TC-UC-22).
func TestRecall_maxTokensTruncation(t *testing.T) {
	st := openTestStoreForRecall(t)
	long := strings.Repeat("fox ", 60)
	for i := 0; i < 10; i++ {
		path := ".claude/skills/long" + string(rune('0'+i)) + "/SKILL.md"
		upsertSkillForRecall(t, st, path, "long"+string(rune('0'+i)), long)
	}

	matches, recErr := recallMatches(context.Background(), st.DB(), recallOptions{
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
	for _, m := range matches {
		if len(m.Summary) > summaryMaxChars {
			t.Errorf("summary length %d > summaryMaxChars %d (mid-truncation suspected)", len(m.Summary), summaryMaxChars)
		}
	}
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
	st := openTestStoreForRecall(t)
	upsertSkillForRecall(t, st, ".claude/skills/foo/SKILL.md", "foo skill", "fox jumps high")
	upsertMemoryForRecall(t, st, "memory/foo.md", "foo memory", "fox flies low")

	matches, recErr := recallMatches(context.Background(), st.DB(), recallOptions{
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
	st := openTestStoreForRecall(t)
	for i := 0; i < 5; i++ {
		path := ".claude/skills/foo" + string(rune('0'+i)) + "/SKILL.md"
		upsertSkillForRecall(t, st, path, "foo", "fox jumps")
	}
	matches, recErr := recallMatches(context.Background(), st.DB(), recallOptions{
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
