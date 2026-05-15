package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

// seedNudgeStore opens a clean store.
func seedNudgeStore(t *testing.T) (string, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	st, openErr := store.Open(dbPath)
	if openErr != nil {
		t.Fatalf("store.Open: %v", openErr)
	}
	t.Cleanup(func() { _ = st.Close() })
	return dbPath, st
}

// testCfg returns a config with TTLDays default (90).
func testCfg() *config.Config {
	return config.DefaultConfig()
}

// nudgeTickOutput mirrors the JSON shape emitted by nudge-tick.
type nudgeTickOutput struct {
	CheckedAt  string                 `json:"checked_at"`
	TTLDays    int                    `json:"ttl_days"`
	Candidates []nudgeTickCandidateJS `json:"candidates"`
}

type nudgeTickCandidateJS struct {
	Kind       string  `json:"kind"`
	Path       string  `json:"path"`
	IndexedAt  string  `json:"indexed_at"`
	LastUsedAt *string `json:"last_used_at"`
	UsageCount int     `json:"usage_count"`
}

// TC-UC-06d (REQ-11): --reset zeroes counter and sets last_nudge_at.
func TestRunNudgeTick_resetZeroesCounter(t *testing.T) {
	dbPath, st := seedNudgeStore(t)
	ctx := context.Background()

	// Pre-set counter to 7 via direct SQL.
	if _, execErr := st.DB().ExecContext(ctx,
		`UPDATE nudge_state SET counter = 7, last_nudge_at = NULL WHERE id = 1`,
	); execErr != nil {
		t.Fatalf("seed counter: %v", execErr)
	}

	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	var stdout, stderr bytes.Buffer
	runErr := runNudgeTick(ctx, nudgeTickOpts{
		DBPath: dbPath,
		Reset:  true,
		Format: "json",
		Config: testCfg(),
		Stdout: &stdout,
		Stderr: &stderr,
		Now:    func() time.Time { return now },
	})
	if runErr != nil {
		t.Fatalf("runNudgeTick: %v", runErr)
	}

	// Stdout must be empty JSON object plus newline.
	if got := stdout.String(); got != "{}\n" {
		t.Errorf("stdout = %q, want \"{}\\n\"", got)
	}

	var (
		counter     int
		lastNudgeAt *string
	)
	if scanErr := st.DB().QueryRowContext(ctx,
		`SELECT counter, last_nudge_at FROM nudge_state WHERE id = 1`,
	).Scan(&counter, &lastNudgeAt); scanErr != nil {
		t.Fatalf("query nudge_state: %v", scanErr)
	}
	if counter != 0 {
		t.Errorf("counter = %d, want 0", counter)
	}
	if lastNudgeAt == nil || *lastNudgeAt == "" {
		t.Errorf("last_nudge_at = nil/empty, want non-empty timestamp")
	}
}

// helper: insert a skill_index row with a forced indexed_at, plus optional skill_usage row.
func insertSkillWithUsage(t *testing.T, st *store.Store, path, indexedAt string, lastUsedAt *string, usageCount int) {
	t.Helper()
	ctx := context.Background()
	if _, execErr := st.DB().ExecContext(ctx,
		`INSERT INTO skill_index(path, title, body, tags, frontmatter_json, indexed_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
		path, "title-"+path, "body", "", "{}", indexedAt,
	); execErr != nil {
		t.Fatalf("insert skill_index: %v", execErr)
	}
	// Always insert a skill_usage row (even if last_used_at is NULL) so the
	// JOIN in ListDeprecationCandidates surfaces the row. Real callers do the
	// same via IncrementSkillUsage, but tests need control over NULL semantics.
	var (
		execErr error
	)
	if lastUsedAt == nil {
		_, execErr = st.DB().ExecContext(ctx,
			`INSERT INTO skill_usage(path, usage_count, last_used_at) VALUES (?, ?, NULL)`,
			path, usageCount,
		)
	} else {
		_, execErr = st.DB().ExecContext(ctx,
			`INSERT INTO skill_usage(path, usage_count, last_used_at) VALUES (?, ?, ?)`,
			path, usageCount, *lastUsedAt,
		)
	}
	if execErr != nil {
		t.Fatalf("insert skill_usage: %v", execErr)
	}
}

// parseTickJSON runs nudgeTick with a deterministic Now and returns the parsed output.
func parseTickJSON(t *testing.T, dbPath string, cfg *config.Config, now time.Time) nudgeTickOutput {
	t.Helper()
	var stdout, stderr bytes.Buffer
	runErr := runNudgeTick(context.Background(), nudgeTickOpts{
		DBPath: dbPath,
		Format: "json",
		Config: cfg,
		Stdout: &stdout,
		Stderr: &stderr,
		Now:    func() time.Time { return now },
	})
	if runErr != nil {
		t.Fatalf("runNudgeTick: %v", runErr)
	}
	var out nudgeTickOutput
	if jsonErr := json.Unmarshal(stdout.Bytes(), &out); jsonErr != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout.String(), jsonErr)
	}
	return out
}

// rfc3339 formats t as RFC3339 UTC.
func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// TC-UC-26 (REQ-12 happy): indexed_at=now-200d, last_used_at=now-100d, TTL=90 → candidate.
func TestRunNudgeTick_listsDeprecationCandidates(t *testing.T) {
	dbPath, st := seedNudgeStore(t)
	cfg := testCfg() // TTLDays=90
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	indexedAt := rfc3339(now.AddDate(0, 0, -200))
	lastUsed := rfc3339(now.AddDate(0, 0, -100))
	insertSkillWithUsage(t, st, ".claude/skills/old/SKILL.md", indexedAt, &lastUsed, 1)

	// And a fresh skill that should NOT be a candidate.
	freshIndexed := rfc3339(now.AddDate(0, 0, -10))
	freshUsed := rfc3339(now.AddDate(0, 0, -1))
	insertSkillWithUsage(t, st, ".claude/skills/fresh/SKILL.md", freshIndexed, &freshUsed, 3)

	out := parseTickJSON(t, dbPath, cfg, now)
	if out.TTLDays != 90 {
		t.Errorf("ttl_days = %d, want 90", out.TTLDays)
	}
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates len = %d, want 1: %+v", len(out.Candidates), out.Candidates)
	}
	c := out.Candidates[0]
	if c.Kind != "skill" {
		t.Errorf("kind = %q, want skill", c.Kind)
	}
	if c.Path != ".claude/skills/old/SKILL.md" {
		t.Errorf("path = %q", c.Path)
	}
	if c.IndexedAt != indexedAt {
		t.Errorf("indexed_at = %q, want %q", c.IndexedAt, indexedAt)
	}
	if c.LastUsedAt == nil || *c.LastUsedAt != lastUsed {
		t.Errorf("last_used_at = %v, want %q", c.LastUsedAt, lastUsed)
	}
	if c.UsageCount != 1 {
		t.Errorf("usage_count = %d, want 1", c.UsageCount)
	}
	if out.CheckedAt == "" {
		t.Errorf("checked_at empty")
	}
}

// TC-UC-27a (boundary): indexed_at = now - (2*TTL - 1)d, last_used_at = now - (TTL+1)d → NOT a candidate.
func TestRunNudgeTick_boundaryCreatedTooRecent(t *testing.T) {
	dbPath, st := seedNudgeStore(t)
	cfg := testCfg()
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	indexedAt := rfc3339(now.AddDate(0, 0, -(2*cfg.TTLDays - 1))) // 179 days ago
	lastUsed := rfc3339(now.AddDate(0, 0, -(cfg.TTLDays + 1)))    // 91 days ago
	insertSkillWithUsage(t, st, ".claude/skills/x/SKILL.md", indexedAt, &lastUsed, 1)

	out := parseTickJSON(t, dbPath, cfg, now)
	if len(out.Candidates) != 0 {
		t.Errorf("candidates len = %d, want 0: %+v", len(out.Candidates), out.Candidates)
	}
}

// TC-UC-27b (boundary inclusive): indexed_at = now - exactly 2*TTL, last_used_at = now - exactly TTL → IS a candidate.
func TestRunNudgeTick_boundaryInclusive(t *testing.T) {
	dbPath, st := seedNudgeStore(t)
	cfg := testCfg()
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	indexedAt := rfc3339(now.AddDate(0, 0, -2*cfg.TTLDays)) // 180 days ago
	lastUsed := rfc3339(now.AddDate(0, 0, -cfg.TTLDays))    // 90 days ago
	insertSkillWithUsage(t, st, ".claude/skills/y/SKILL.md", indexedAt, &lastUsed, 2)

	out := parseTickJSON(t, dbPath, cfg, now)
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates len = %d, want 1: %+v", len(out.Candidates), out.Candidates)
	}
	if out.Candidates[0].Path != ".claude/skills/y/SKILL.md" {
		t.Errorf("path = %q", out.Candidates[0].Path)
	}
}

// TC-UC-27c (boundary): indexed_at = now - (2*TTL+1), last_used_at = now - (TTL-1) → NOT a candidate (last_used too recent).
func TestRunNudgeTick_boundaryUsedTooRecent(t *testing.T) {
	dbPath, st := seedNudgeStore(t)
	cfg := testCfg()
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	indexedAt := rfc3339(now.AddDate(0, 0, -(2*cfg.TTLDays + 1))) // 181 days ago
	lastUsed := rfc3339(now.AddDate(0, 0, -(cfg.TTLDays - 1)))    // 89 days ago
	insertSkillWithUsage(t, st, ".claude/skills/z/SKILL.md", indexedAt, &lastUsed, 1)

	out := parseTickJSON(t, dbPath, cfg, now)
	if len(out.Candidates) != 0 {
		t.Errorf("candidates len = %d, want 0: %+v", len(out.Candidates), out.Candidates)
	}
}

// Bonus coverage: NULL last_used_at on a row with indexed_at < now-2*TTL → IS a candidate
// (treats NULL as "never used"). This is implied by REQ-12 wording in the task spec.
func TestRunNudgeTick_neverUsedOldRowIsCandidate(t *testing.T) {
	dbPath, st := seedNudgeStore(t)
	cfg := testCfg()
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	indexedAt := rfc3339(now.AddDate(0, 0, -2*cfg.TTLDays-5))
	insertSkillWithUsage(t, st, ".claude/skills/never/SKILL.md", indexedAt, nil, 0)

	out := parseTickJSON(t, dbPath, cfg, now)
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates len = %d, want 1: %+v", len(out.Candidates), out.Candidates)
	}
	if out.Candidates[0].LastUsedAt != nil {
		t.Errorf("last_used_at = %v, want nil", out.Candidates[0].LastUsedAt)
	}
}

// --ttl-days override beats config.
func TestRunNudgeTick_ttlOverride(t *testing.T) {
	dbPath, st := seedNudgeStore(t)
	cfg := testCfg() // TTLDays=90
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	// 50d old indexed, 25d unused — not a candidate with TTL=90 (needs >=180d index)
	// but IS a candidate with TTL=20 (needs >=40d index, >=20d unused).
	indexedAt := rfc3339(now.AddDate(0, 0, -50))
	lastUsed := rfc3339(now.AddDate(0, 0, -25))
	insertSkillWithUsage(t, st, ".claude/skills/k/SKILL.md", indexedAt, &lastUsed, 1)

	// Default TTL=90 → no candidates.
	out90 := parseTickJSON(t, dbPath, cfg, now)
	if len(out90.Candidates) != 0 {
		t.Errorf("TTL=90 candidates len = %d, want 0", len(out90.Candidates))
	}

	// Override TTL=20 → candidate.
	var stdout, stderr bytes.Buffer
	runErr := runNudgeTick(context.Background(), nudgeTickOpts{
		DBPath:  dbPath,
		Format:  "json",
		Config:  cfg,
		TTLDays: 20,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Now:     func() time.Time { return now },
	})
	if runErr != nil {
		t.Fatalf("runNudgeTick: %v", runErr)
	}
	var out20 nudgeTickOutput
	if jsonErr := json.Unmarshal(stdout.Bytes(), &out20); jsonErr != nil {
		t.Fatalf("unmarshal: %v", jsonErr)
	}
	if out20.TTLDays != 20 {
		t.Errorf("ttl_days = %d, want 20 (override)", out20.TTLDays)
	}
	if len(out20.Candidates) != 1 {
		t.Errorf("TTL=20 candidates len = %d, want 1: %+v", len(out20.Candidates), out20.Candidates)
	}
}

// Memory rows also surface as candidates with kind="memory".
func TestRunNudgeTick_includesMemoryCandidates(t *testing.T) {
	dbPath, st := seedNudgeStore(t)
	cfg := testCfg()
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	indexedAt := rfc3339(now.AddDate(0, 0, -300))
	lastUsed := rfc3339(now.AddDate(0, 0, -100))

	if _, execErr := st.DB().ExecContext(ctx,
		`INSERT INTO memory_index(path, title, body, frontmatter_json, indexed_at) VALUES (?, ?, ?, ?, ?)`,
		"memory/old.md", "old", "body", "{}", indexedAt,
	); execErr != nil {
		t.Fatalf("insert memory_index: %v", execErr)
	}
	if _, execErr := st.DB().ExecContext(ctx,
		`INSERT INTO memory_usage(path, usage_count, last_used_at) VALUES (?, ?, ?)`,
		"memory/old.md", 1, lastUsed,
	); execErr != nil {
		t.Fatalf("insert memory_usage: %v", execErr)
	}

	out := parseTickJSON(t, dbPath, cfg, now)
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates len = %d, want 1", len(out.Candidates))
	}
	if out.Candidates[0].Kind != "memory" {
		t.Errorf("kind = %q, want memory", out.Candidates[0].Kind)
	}
	if out.Candidates[0].Path != "memory/old.md" {
		t.Errorf("path = %q", out.Candidates[0].Path)
	}
}
