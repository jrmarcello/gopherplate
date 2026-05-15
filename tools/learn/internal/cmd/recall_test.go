package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

// seedRecallStore opens a fresh store at t.TempDir and seeds skill/memory rows
// for recall tests. Returns the DB path.
func seedRecallStore(t *testing.T, skills, memories map[string]string) string {
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
	for path, body := range skills {
		if upsertErr := st.UpsertSkillIndex(ctx, store.SkillIndexEntry{
			Path:            path,
			Title:           filepath.Base(filepath.Dir(path)),
			Body:            body,
			Tags:            "",
			FrontmatterJSON: "{}",
			IndexedAt:       now,
		}); upsertErr != nil {
			t.Fatalf("UpsertSkillIndex: %v", upsertErr)
		}
	}
	for path, body := range memories {
		if upsertErr := st.UpsertMemoryIndex(ctx, store.MemoryIndexEntry{
			Path:            path,
			Title:           filepath.Base(path),
			Body:            body,
			FrontmatterJSON: "{}",
			IndexedAt:       now,
		}); upsertErr != nil {
			t.Fatalf("UpsertMemoryIndex: %v", upsertErr)
		}
	}
	return dbPath
}

// TC-UC-19 (REQ-14): recall happy path with JSON format returns top-K matches.
func TestRunRecall_jsonHappyPath(t *testing.T) {
	dbPath := seedRecallStore(t, map[string]string{
		".claude/skills/foo/SKILL.md": "the quick brown fox jumps over the lazy dog",
		".claude/skills/bar/SKILL.md": "fox runs across the field",
		".claude/skills/baz/SKILL.md": "elephant in the room",
	}, nil)

	var stdout, stderr bytes.Buffer
	runErr := runRecall(context.Background(), recallOpts{
		Prompt:    "fox jumps over the lazy dog quickly",
		DBPath:    dbPath,
		TopK:      2,
		MaxTokens: 10000,
		MinScore:  0.0,
		Format:    "json",
		Stdout:    &stdout,
		Stderr:    &stderr,
		Config:    config.DefaultConfig(),
	})
	if runErr != nil {
		t.Fatalf("runRecall: %v", runErr)
	}
	var matches []map[string]any
	if jsonErr := json.Unmarshal(stdout.Bytes(), &matches); jsonErr != nil {
		t.Fatalf("json.Unmarshal: %v\nstdout=%q", jsonErr, stdout.String())
	}
	if len(matches) != 2 {
		t.Errorf("len(matches) = %d, want 2", len(matches))
	}
}

// TC-UC-19a (REQ-14a): system-reminder format matches the canonical template.
func TestRunRecall_systemReminderFormat(t *testing.T) {
	dbPath := seedRecallStore(t, map[string]string{
		".claude/skills/foo/SKILL.md": "fox jumps over the lazy dog",
		".claude/skills/bar/SKILL.md": "fox flies high above the cloud",
	}, nil)

	var stdout, stderr bytes.Buffer
	runErr := runRecall(context.Background(), recallOpts{
		Prompt:    "fox flies above the cloud at high altitude",
		DBPath:    dbPath,
		TopK:      2,
		MaxTokens: 10000,
		MinScore:  0.0,
		Format:    "system-reminder",
		Stdout:    &stdout,
		Stderr:    &stderr,
		Config:    config.DefaultConfig(),
	})
	if runErr != nil {
		t.Fatalf("runRecall: %v", runErr)
	}
	got := stdout.String()
	// Template:
	//   <system-reminder>\nLearning loop recall — N relevant artifacts:\n
	//   - <path> (<kind>, score=<X>): <summary>\n
	//   ...
	//   Read the file(s) above if relevant; ignore otherwise.\n</system-reminder>\n
	template := regexp.MustCompile(
		`^<system-reminder>\nLearning loop recall — (\d+) relevant artifacts:\n` +
			`((?:- [^\n]+ \([a-z]+, score=\d+\.\d{2}\): [^\n]*\n)+)` +
			`Read the file\(s\) above if relevant; ignore otherwise\.\n` +
			`</system-reminder>\n$`,
	)
	matches := template.FindStringSubmatch(got)
	if matches == nil {
		t.Fatalf("output does not match REQ-14a template:\n%s", got)
	}
	// Verify N in header equals actual count of dash-lines: a mutant that
	// emits "99 relevant artifacts" with only 1 line would slip past the
	// regex alone. Score=0.00 IS allowed here because the test deliberately
	// sets MinScore=0 so we can exercise the full template structure even
	// with FTS5's degenerate-score edge cases; the score floor is tested
	// separately in TestRunRecall_minScore* boundary tests.
	declaredN, _ := strconv.Atoi(matches[1])
	actualN := strings.Count(matches[2], "\n- ")
	if matches[2] != "" {
		actualN++ // first line doesn't have the leading "\n- " separator.
	}
	if declaredN != actualN {
		t.Errorf("header declares N=%d artifacts but body has %d lines:\n%s", declaredN, actualN, got)
	}
}

// TC-UC-20 (REQ-14): short prompt below MinPromptLenForRecall produces no
// output and exits 0.
func TestRunRecall_shortPromptNoOp(t *testing.T) {
	dbPath := seedRecallStore(t, map[string]string{
		".claude/skills/foo/SKILL.md": "fox",
	}, nil)
	cfg := config.DefaultConfig()
	cfg.MinPromptLenForRecall = 20

	var stdout, stderr bytes.Buffer
	runErr := runRecall(context.Background(), recallOpts{
		Prompt:    strings.Repeat("a", cfg.MinPromptLenForRecall-1),
		DBPath:    dbPath,
		TopK:      cfg.RecallTopK,
		MaxTokens: cfg.RecallMaxTokens,
		MinScore:  cfg.RecallMinScore,
		Format:    "system-reminder",
		Stdout:    &stdout,
		Stderr:    &stderr,
		Config:    cfg,
	})
	if runErr != nil {
		t.Fatalf("runRecall: %v", runErr)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty for short prompt, got: %q", stdout.String())
	}
}

// TC-UC-20a (boundary inclusive): exactly MinPromptLenForRecall chars triggers recall.
func TestRunRecall_minPromptLenBoundaryInclusive(t *testing.T) {
	dbPath := seedRecallStore(t, map[string]string{
		".claude/skills/foo/SKILL.md": "matching content with the magic word fox",
	}, nil)
	cfg := config.DefaultConfig()
	cfg.MinPromptLenForRecall = 20

	// Exactly 20 chars; "fox magic word stuff" -> tokens include "fox".
	prompt := "fox magic word stuff"
	if len(prompt) != cfg.MinPromptLenForRecall {
		t.Fatalf("test setup: prompt len = %d, want %d", len(prompt), cfg.MinPromptLenForRecall)
	}

	var stdout, stderr bytes.Buffer
	runErr := runRecall(context.Background(), recallOpts{
		Prompt:    prompt,
		DBPath:    dbPath,
		TopK:      cfg.RecallTopK,
		MaxTokens: cfg.RecallMaxTokens,
		MinScore:  0.0,
		Format:    "json",
		Stdout:    &stdout,
		Stderr:    &stderr,
		Config:    cfg,
	})
	if runErr != nil {
		t.Fatalf("runRecall: %v", runErr)
	}
	// Recall ran => JSON array (possibly empty) emitted to stdout. The key
	// assertion: stdout is non-empty (i.e. recall didn't short-circuit).
	if stdout.Len() == 0 {
		t.Errorf("stdout should be non-empty (recall ran), got empty")
	}
}

// TC-UC-22c (REQ-14): --top-k=0 is a usage error.
func TestRunRecall_topKZeroUsageError(t *testing.T) {
	dbPath := seedRecallStore(t, nil, nil)
	var stdout, stderr bytes.Buffer
	runErr := runRecall(context.Background(), recallOpts{
		Prompt:    "a long enough prompt to bypass min len",
		DBPath:    dbPath,
		TopK:      0,
		MaxTokens: 500,
		Format:    "json",
		Stdout:    &stdout,
		Stderr:    &stderr,
		Config:    config.DefaultConfig(),
	})
	if runErr == nil {
		t.Fatal("expected usage error, got nil")
	}
	var usageErr *learnerr.UsageError
	if !errors.As(runErr, &usageErr) {
		t.Errorf("expected *UsageError, got %T", runErr)
	}
}

// TC-UC-22d (REQ-14): --max-tokens=0 is a usage error.
func TestRunRecall_maxTokensZeroUsageError(t *testing.T) {
	dbPath := seedRecallStore(t, nil, nil)
	var stdout, stderr bytes.Buffer
	runErr := runRecall(context.Background(), recallOpts{
		Prompt:    "a long enough prompt to bypass min len",
		DBPath:    dbPath,
		TopK:      3,
		MaxTokens: 0,
		Format:    "json",
		Stdout:    &stdout,
		Stderr:    &stderr,
		Config:    config.DefaultConfig(),
	})
	if runErr == nil {
		t.Fatal("expected usage error, got nil")
	}
	var usageErr *learnerr.UsageError
	if !errors.As(runErr, &usageErr) {
		t.Errorf("expected *UsageError, got %T", runErr)
	}
}

// TC-UC-23a (REQ-16): --kind=invalid is a usage error.
func TestRunRecall_invalidKindUsageError(t *testing.T) {
	dbPath := seedRecallStore(t, nil, nil)
	var stdout, stderr bytes.Buffer
	runErr := runRecall(context.Background(), recallOpts{
		Prompt:     "a long enough prompt to bypass min len",
		DBPath:     dbPath,
		TopK:       3,
		MaxTokens:  500,
		KindFilter: "invalid",
		Format:     "json",
		Stdout:     &stdout,
		Stderr:     &stderr,
		Config:     config.DefaultConfig(),
	})
	if runErr == nil {
		t.Fatal("expected usage error, got nil")
	}
	var usageErr *learnerr.UsageError
	if !errors.As(runErr, &usageErr) {
		t.Errorf("expected *UsageError, got %T", runErr)
	}
}

// TC-UC-23 (REQ-16): --kind=memory filters to memory_fts results.
func TestRunRecall_kindMemoryFilter(t *testing.T) {
	dbPath := seedRecallStore(t,
		map[string]string{".claude/skills/foo/SKILL.md": "fox jumps high"},
		map[string]string{"memory/bar.md": "fox flies low across the field"},
	)

	var stdout, stderr bytes.Buffer
	runErr := runRecall(context.Background(), recallOpts{
		Prompt:     "fox flies low across the wide open field",
		DBPath:     dbPath,
		TopK:       10,
		MaxTokens:  10000,
		MinScore:   0.0,
		KindFilter: "memory",
		Format:     "json",
		Stdout:     &stdout,
		Stderr:     &stderr,
		Config:     config.DefaultConfig(),
	})
	if runErr != nil {
		t.Fatalf("runRecall: %v", runErr)
	}
	var matches []map[string]any
	if jsonErr := json.Unmarshal(stdout.Bytes(), &matches); jsonErr != nil {
		t.Fatalf("json.Unmarshal: %v", jsonErr)
	}
	for _, m := range matches {
		if m["Kind"] != "memory" {
			t.Errorf("unexpected kind=%v in %+v", m["Kind"], m)
		}
	}
}

// TC-UC-15 (no matches): when no rows match, system-reminder format produces
// empty stdout (REQ-15).
func TestRunRecall_noMatchesSilent(t *testing.T) {
	dbPath := seedRecallStore(t, map[string]string{
		".claude/skills/foo/SKILL.md": "elephant in the room",
	}, nil)
	var stdout, stderr bytes.Buffer
	runErr := runRecall(context.Background(), recallOpts{
		Prompt:    "completely unrelated nonsense xyzqwertyuiop",
		DBPath:    dbPath,
		TopK:      3,
		MaxTokens: 500,
		MinScore:  0.0,
		Format:    "system-reminder",
		Stdout:    &stdout,
		Stderr:    &stderr,
		Config:    config.DefaultConfig(),
	})
	if runErr != nil {
		t.Fatalf("runRecall: %v", runErr)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout on no matches, got: %q", stdout.String())
	}
}
