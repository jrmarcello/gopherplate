package learn

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

// statsJSON mirrors the documented schema; the test only asserts a subset of
// fields, but using a typed struct verifies the wire schema.
type statsJSON struct {
	Counts struct {
		Events     int `json:"events"`
		Patterns   int `json:"patterns"`
		Candidates int `json:"candidates"`
		Decisions  int `json:"decisions"`
		Skills     int `json:"skills"`
		Memory     int `json:"memory"`
	} `json:"counts"`
	LastExtractAt  string `json:"last_extract_at"`
	LastNudgeAt    string `json:"last_nudge_at"`
	NudgeCounter   int    `json:"nudge_counter"`
	TopSkillsByUse []struct {
		Path string `json:"path"`
		Uses int    `json:"uses"`
	} `json:"top_skills_by_use"`
	BottomSkillsByUse []struct {
		Path string `json:"path"`
		Uses int    `json:"uses"`
	} `json:"bottom_skills_by_use"`
	DBSizeBytes int64 `json:"db_size_bytes"`
}

// TC-UC-28 (REQ-23 happy): populated DB -> JSON with non-zero counts and
// at least one top_skills row.
func TestRunStats_populatedDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")

	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	ctx := context.Background()
	db := st.DB()

	// Insert one event
	if _, execErr := db.ExecContext(ctx,
		`INSERT INTO events (spec_path, session_id, completed_at) VALUES (?, ?, ?)`,
		"spec.md", "sess-1", "2026-05-14T00:00:00Z"); execErr != nil {
		t.Fatalf("insert event: %v", execErr)
	}
	// Insert one pattern
	if _, execErr := db.ExecContext(ctx,
		`INSERT INTO patterns (id, kind, signature, frequency, first_seen_at, last_seen_at, score)
		 VALUES (1, 'tool', 'sig-1', 3, '2026-05-13T00:00:00Z', '2026-05-14T00:00:00Z', 0.5)`); execErr != nil {
		t.Fatalf("insert pattern: %v", execErr)
	}
	// Insert one candidate
	if _, execErr := db.ExecContext(ctx,
		`INSERT INTO candidates (id, pattern_id, contexts_json, score, created_at)
		 VALUES (1, 1, '[]', 0.7, '2026-05-14T01:00:00Z')`); execErr != nil {
		t.Fatalf("insert candidate: %v", execErr)
	}
	// Insert one decision
	if _, execErr := db.ExecContext(ctx,
		`INSERT INTO decisions (id, candidate_signature, action, decided_at)
		 VALUES (1, 'sig-1', 'new-skill', '2026-05-14T02:00:00Z')`); execErr != nil {
		t.Fatalf("insert decision: %v", execErr)
	}
	// Insert two skills + usage rows
	if _, execErr := db.ExecContext(ctx,
		`INSERT INTO skill_index (path, title, body, frontmatter_json, indexed_at)
		 VALUES ('skills/a.md', 'A', 'body', '{}', '2026-05-14T00:00:00Z'),
		        ('skills/b.md', 'B', 'body', '{}', '2026-05-14T00:00:00Z')`); execErr != nil {
		t.Fatalf("insert skills: %v", execErr)
	}
	if _, execErr := db.ExecContext(ctx,
		`INSERT INTO skill_usage (path, usage_count) VALUES ('skills/a.md', 10), ('skills/b.md', 2)`); execErr != nil {
		t.Fatalf("insert skill_usage: %v", execErr)
	}
	// Insert one memory
	if _, execErr := db.ExecContext(ctx,
		`INSERT INTO memory_index (path, title, body, frontmatter_json, indexed_at)
		 VALUES ('memory/m.md', 'M', 'b', '{}', '2026-05-14T00:00:00Z')`); execErr != nil {
		t.Fatalf("insert memory: %v", execErr)
	}
	// Set nudge_state
	if _, execErr := db.ExecContext(ctx,
		`UPDATE nudge_state SET counter=7, last_nudge_at='2026-05-14T03:00:00Z' WHERE id=1`); execErr != nil {
		t.Fatalf("update nudge_state: %v", execErr)
	}
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close store: %v", closeErr)
	}

	var stdout, stderr bytes.Buffer
	runErr := runStats(ctx, statsOpts{DBPath: dbPath, Stdout: &stdout, Stderr: &stderr})
	if runErr != nil {
		t.Fatalf("runStats: %v (stderr=%q)", runErr, stderr.String())
	}

	var got statsJSON
	if jsonErr := json.Unmarshal(stdout.Bytes(), &got); jsonErr != nil {
		t.Fatalf("unmarshal stdout JSON: %v\noutput=%q", jsonErr, stdout.String())
	}

	if got.Counts.Events != 1 {
		t.Errorf("events: got %d, want 1", got.Counts.Events)
	}
	if got.Counts.Patterns != 1 {
		t.Errorf("patterns: got %d, want 1", got.Counts.Patterns)
	}
	if got.Counts.Candidates != 1 {
		t.Errorf("candidates: got %d, want 1", got.Counts.Candidates)
	}
	if got.Counts.Decisions != 1 {
		t.Errorf("decisions: got %d, want 1", got.Counts.Decisions)
	}
	if got.Counts.Skills != 2 {
		t.Errorf("skills: got %d, want 2", got.Counts.Skills)
	}
	if got.Counts.Memory != 1 {
		t.Errorf("memory: got %d, want 1", got.Counts.Memory)
	}
	if got.LastExtractAt != "2026-05-14T01:00:00Z" {
		t.Errorf("last_extract_at: got %q, want %q", got.LastExtractAt, "2026-05-14T01:00:00Z")
	}
	if got.LastNudgeAt != "2026-05-14T03:00:00Z" {
		t.Errorf("last_nudge_at: got %q, want %q", got.LastNudgeAt, "2026-05-14T03:00:00Z")
	}
	if got.NudgeCounter != 7 {
		t.Errorf("nudge_counter: got %d, want 7", got.NudgeCounter)
	}
	if len(got.TopSkillsByUse) == 0 {
		t.Fatalf("expected top_skills_by_use to be non-empty")
	}
	if got.TopSkillsByUse[0].Path != "skills/a.md" || got.TopSkillsByUse[0].Uses != 10 {
		t.Errorf("top[0]: got %+v, want {skills/a.md, 10}", got.TopSkillsByUse[0])
	}
	if len(got.BottomSkillsByUse) == 0 || got.BottomSkillsByUse[0].Path != "skills/b.md" {
		t.Errorf("bottom: got %+v, want first=skills/b.md", got.BottomSkillsByUse)
	}
	if got.DBSizeBytes <= 0 {
		t.Errorf("db_size_bytes: got %d, want > 0", got.DBSizeBytes)
	}
}

// TC-UC-29 (REQ-23 edge): empty DB.
func TestRunStats_emptyDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close store: %v", closeErr)
	}

	var stdout, stderr bytes.Buffer
	runErr := runStats(context.Background(), statsOpts{DBPath: dbPath, Stdout: &stdout, Stderr: &stderr})
	if runErr != nil {
		t.Fatalf("runStats: %v (stderr=%q)", runErr, stderr.String())
	}

	var got statsJSON
	if jsonErr := json.Unmarshal(stdout.Bytes(), &got); jsonErr != nil {
		t.Fatalf("unmarshal: %v\noutput=%q", jsonErr, stdout.String())
	}
	if got.Counts.Events != 0 || got.Counts.Patterns != 0 || got.Counts.Candidates != 0 ||
		got.Counts.Decisions != 0 || got.Counts.Skills != 0 || got.Counts.Memory != 0 {
		t.Errorf("expected all counts zero, got %+v", got.Counts)
	}
	if got.LastExtractAt != "" {
		t.Errorf("last_extract_at: got %q, want empty", got.LastExtractAt)
	}
	if got.LastNudgeAt != "" {
		t.Errorf("last_nudge_at: got %q, want empty", got.LastNudgeAt)
	}
	if got.NudgeCounter != 0 {
		t.Errorf("nudge_counter: got %d, want 0", got.NudgeCounter)
	}
	if len(got.TopSkillsByUse) != 0 {
		t.Errorf("top_skills_by_use: got %v, want empty", got.TopSkillsByUse)
	}
	if len(got.BottomSkillsByUse) != 0 {
		t.Errorf("bottom_skills_by_use: got %v, want empty", got.BottomSkillsByUse)
	}
}

// TC-UC-30b (REQ-22): `stats --help` returns exit 0 (no error).
func TestStatsCmd_helpFlag(t *testing.T) {
	cmd := newStatsCmd()
	cmd.SetArgs([]string{"--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if execErr := cmd.Execute(); execErr != nil {
		t.Fatalf("stats --help returned error: %v", execErr)
	}
	if out.Len() == 0 {
		t.Errorf("expected help output, got empty")
	}
}

// TC-UC-31a (REQ-22): bad DB path -> RuntimeError (exit 2).
func TestRunStats_badDBPath_returnsRuntimeError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runErr := runStats(context.Background(), statsOpts{
		DBPath: "/nonexistent/dir/does/not/exist/db.sqlite",
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if runErr == nil {
		t.Fatalf("expected error, got nil")
	}
	var re *runtimeError
	if !errors.As(runErr, &re) {
		t.Errorf("expected *runtimeError, got %T: %v", runErr, runErr)
	}
}
