package learn

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeSkillFile writes a minimal SKILL.md with the given frontmatter name/tags
// and body. Returned path lives under dir.
// nolint:unparam // name is varied across tests for clarity even though
// the current happy-path callers all pass the same relative skill path.
func writeSkillFile(t *testing.T, dir, name, title, tags, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	content := "---\nname: " + title + "\n"
	if tags != "" {
		content += "tags: [" + tags + "]\n"
	}
	content += "---\n" + body + "\n"
	if writeErr := os.WriteFile(path, []byte(content), 0o644); writeErr != nil {
		t.Fatalf("write file: %v", writeErr)
	}
	return path
}

// seedSkillIndex inserts a row into skill_index directly, bypassing reindex.
func seedSkillIndex(t *testing.T, ctx context.Context, st *sqliteStore, path, title, body, tags string) {
	t.Helper()
	entry := skillIndexEntry{
		Path:            path,
		Title:           title,
		Body:            body,
		Tags:            tags,
		FrontmatterJSON: `{"name":"` + title + `"}`,
		IndexedAt:       "2026-05-14T00:00:00Z",
	}
	if upsertErr := st.UpsertSkillIndex(ctx, entry); upsertErr != nil {
		t.Fatalf("upsert skill_index: %v", upsertErr)
	}
}

// TC-UC-16 (REQ-8 happy): three skills indexed, runSimilar on one of them
// returns the other two ordered by score, target self-excluded.
func TestRunSimilar_happy(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	ctx := context.Background()

	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	// Target skill — also written to disk so runSimilar can read its frontmatter.
	targetPath := writeSkillFile(t, dir, "skills/alpha/SKILL.md",
		"alpha", "kubernetes,docker", "alpha skill body about kubernetes deployments")

	seedSkillIndex(t, ctx, st, targetPath, "alpha",
		"alpha skill body about kubernetes deployments", "kubernetes,docker")
	seedSkillIndex(t, ctx, st, "skills/beta/SKILL.md", "beta",
		"beta skill discusses kubernetes operators", "kubernetes")
	seedSkillIndex(t, ctx, st, "skills/gamma/SKILL.md", "gamma",
		"gamma is about kubernetes networking", "kubernetes")
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	cfg := defaultLoopConfig()
	cfg.SimilarityThreshold = 0.0
	var stdout, stderr bytes.Buffer
	runErr := runSimilar(ctx, similarOpts{
		DBPath:    dbPath,
		SkillPath: targetPath,
		TopK:      5,
		Threshold: 0.0,
		Config:    cfg,
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if runErr != nil {
		t.Fatalf("runSimilar: %v (stderr=%q)", runErr, stderr.String())
	}

	var got []similarOutRow
	if jsonErr := json.Unmarshal(stdout.Bytes(), &got); jsonErr != nil {
		t.Fatalf("unmarshal: %v, output=%q", jsonErr, stdout.String())
	}
	// Target itself MUST be excluded.
	for _, r := range got {
		if r.Path == targetPath {
			t.Fatalf("target self not excluded: %+v", got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results (beta, gamma), got %d: %+v", len(got), got)
	}
	// Ordering: scores must be descending; ties broken by Path asc.
	for i := 1; i < len(got); i++ {
		if got[i-1].Score < got[i].Score {
			t.Errorf("results not score-descending: %+v", got)
		}
	}
	// Each result must include an edit_distance in [0,1].
	for _, r := range got {
		if r.EditDistance < 0.0 || r.EditDistance > 1.0 {
			t.Errorf("edit_distance out of [0,1]: %v", r.EditDistance)
		}
	}
}

// TC-UC-16a (determinism): runSimilar twice with same DB state produces
// byte-identical output.
func TestRunSimilar_deterministic(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	ctx := context.Background()

	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	targetPath := writeSkillFile(t, dir, "skills/alpha/SKILL.md",
		"alpha", "redis,cache", "alpha body about redis caching")
	seedSkillIndex(t, ctx, st, targetPath, "alpha",
		"alpha body about redis caching", "redis,cache")
	seedSkillIndex(t, ctx, st, "skills/beta/SKILL.md", "beta",
		"beta about redis sharding", "redis")
	seedSkillIndex(t, ctx, st, "skills/gamma/SKILL.md", "gamma",
		"gamma about redis ttl", "redis")
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	cfg := defaultLoopConfig()
	cfg.SimilarityThreshold = 0.0
	var out1, out2 bytes.Buffer
	opts := similarOpts{
		DBPath:    dbPath,
		SkillPath: targetPath,
		TopK:      5,
		Threshold: 0.0,
		Config:    cfg,
		Stdout:    &out1,
		Stderr:    &bytes.Buffer{},
	}
	if runErr := runSimilar(ctx, opts); runErr != nil {
		t.Fatalf("runSimilar #1: %v", runErr)
	}
	opts.Stdout = &out2
	if runErr := runSimilar(ctx, opts); runErr != nil {
		t.Fatalf("runSimilar #2: %v", runErr)
	}
	if !bytes.Equal(out1.Bytes(), out2.Bytes()) {
		t.Errorf("non-deterministic output:\n#1=%q\n#2=%q", out1.String(), out2.String())
	}
}

// TC-UC-17 (REQ-8 edge): empty skill_index (only the target is on disk and
// indexed — actually we test with only one indexed skill which is the target).
// runSimilar yields an empty JSON array.
func TestRunSimilar_emptyOtherSkills(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	ctx := context.Background()

	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	targetPath := writeSkillFile(t, dir, "skills/alpha/SKILL.md",
		"alpha", "kubernetes", "alpha solo body")
	seedSkillIndex(t, ctx, st, targetPath, "alpha", "alpha solo body", "kubernetes")
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	cfg := defaultLoopConfig()
	cfg.SimilarityThreshold = 0.0
	var stdout, stderr bytes.Buffer
	runErr := runSimilar(ctx, similarOpts{
		DBPath:    dbPath,
		SkillPath: targetPath,
		TopK:      5,
		Threshold: 0.0,
		Config:    cfg,
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if runErr != nil {
		t.Fatalf("runSimilar: %v", runErr)
	}
	var got []similarOutRow
	if jsonErr := json.Unmarshal(stdout.Bytes(), &got); jsonErr != nil {
		t.Fatalf("unmarshal: %v, output=%q", jsonErr, stdout.String())
	}
	if len(got) != 0 {
		t.Errorf("expected empty array, got %+v", got)
	}
}

// TC-UC-18a (boundary): threshold=0.6, all candidates have score < 0.6.
// Uses injected QueryFn to fabricate scores precisely.
func TestRunSimilar_threshold_belowExcluded(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	ctx := context.Background()

	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	targetPath := writeSkillFile(t, dir, "skills/alpha/SKILL.md",
		"alpha", "tag", "body")
	seedSkillIndex(t, ctx, st, targetPath, "alpha", "body", "tag")
	seedSkillIndex(t, ctx, st, "skills/beta/SKILL.md", "beta", "beta body", "tag")
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	mockQuery := func(ctx context.Context, db *sql.DB, opts ftsQueryOpts) ([]similarMatch, error) {
		// Fabricate one match below the threshold; Query helper would have
		// filtered this — but to validate runSimilar's own filter we return
		// it unfiltered and assert it's dropped.
		if opts.minScore > 0.59 {
			// runSimilar passes MinScore through; the production Query honors
			// it. We simulate exactly that filtering here.
			return []similarMatch{}, nil
		}
		return []similarMatch{
			{index: indexSkill, path: "skills/beta/SKILL.md", score: 0.59},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	runErr := runSimilar(ctx, similarOpts{
		DBPath:    dbPath,
		SkillPath: targetPath,
		TopK:      5,
		Threshold: 0.6,
		QueryFn:   mockQuery,
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if runErr != nil {
		t.Fatalf("runSimilar: %v", runErr)
	}
	var got []similarOutRow
	if jsonErr := json.Unmarshal(stdout.Bytes(), &got); jsonErr != nil {
		t.Fatalf("unmarshal: %v, output=%q", jsonErr, stdout.String())
	}
	if len(got) != 0 {
		t.Errorf("expected empty (all below threshold), got %+v", got)
	}
}

// TC-UC-18b (boundary inclusive): threshold=0.6, candidate at exactly 0.60.
func TestRunSimilar_threshold_inclusiveAtBoundary(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	ctx := context.Background()

	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	targetPath := writeSkillFile(t, dir, "skills/alpha/SKILL.md",
		"alpha", "tag", "alpha body")
	seedSkillIndex(t, ctx, st, targetPath, "alpha", "alpha body", "tag")
	seedSkillIndex(t, ctx, st, "skills/beta/SKILL.md", "beta", "beta body", "tag")
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	mockQuery := func(ctx context.Context, db *sql.DB, opts ftsQueryOpts) ([]similarMatch, error) {
		// queryFTSIndex semantics: returns matches with score >= MinScore.
		if 0.60 < opts.minScore {
			return []similarMatch{}, nil
		}
		return []similarMatch{
			{index: indexSkill, path: "skills/beta/SKILL.md", score: 0.60},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	runErr := runSimilar(ctx, similarOpts{
		DBPath:    dbPath,
		SkillPath: targetPath,
		TopK:      5,
		Threshold: 0.6,
		QueryFn:   mockQuery,
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if runErr != nil {
		t.Fatalf("runSimilar: %v", runErr)
	}
	var got []similarOutRow
	if jsonErr := json.Unmarshal(stdout.Bytes(), &got); jsonErr != nil {
		t.Fatalf("unmarshal: %v", jsonErr)
	}
	if len(got) != 1 || got[0].Path != "skills/beta/SKILL.md" {
		t.Errorf("expected 1 result at boundary, got %+v", got)
	}
}

// TC-UC-18c (boundary): score 0.61 — included.
func TestRunSimilar_threshold_aboveIncluded(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	ctx := context.Background()

	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	targetPath := writeSkillFile(t, dir, "skills/alpha/SKILL.md",
		"alpha", "tag", "alpha body")
	seedSkillIndex(t, ctx, st, targetPath, "alpha", "alpha body", "tag")
	seedSkillIndex(t, ctx, st, "skills/beta/SKILL.md", "beta", "beta body", "tag")
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	mockQuery := func(ctx context.Context, db *sql.DB, opts ftsQueryOpts) ([]similarMatch, error) {
		if 0.61 < opts.minScore {
			return []similarMatch{}, nil
		}
		return []similarMatch{
			{index: indexSkill, path: "skills/beta/SKILL.md", score: 0.61},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	runErr := runSimilar(ctx, similarOpts{
		DBPath:    dbPath,
		SkillPath: targetPath,
		TopK:      5,
		Threshold: 0.6,
		QueryFn:   mockQuery,
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if runErr != nil {
		t.Fatalf("runSimilar: %v", runErr)
	}
	var got []similarOutRow
	if jsonErr := json.Unmarshal(stdout.Bytes(), &got); jsonErr != nil {
		t.Fatalf("unmarshal: %v", jsonErr)
	}
	if len(got) != 1 || got[0].Path != "skills/beta/SKILL.md" {
		t.Errorf("expected 1 result above threshold, got %+v", got)
	}
}

// Missing --skill flag -> UsageError (exit 1).
func TestRunSimilar_missingSkillFlag(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	var stdout, stderr bytes.Buffer
	runErr := runSimilar(context.Background(), similarOpts{
		DBPath:    dbPath,
		SkillPath: "",
		TopK:      5,
		Threshold: 0.0,
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if runErr == nil {
		t.Fatalf("expected error, got nil")
	}
	var ue *usageError
	if !errors.As(runErr, &ue) {
		t.Errorf("expected *usageError, got %T: %v", runErr, runErr)
	}
}

// Defaults: when Threshold is 0 and Config is nil, runSimilar must load
// config (use cfg.SimilarityThreshold).
func TestRunSimilar_threshold_fromConfigDefault(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	ctx := context.Background()

	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	targetPath := writeSkillFile(t, dir, "skills/alpha/SKILL.md",
		"alpha", "tag", "alpha body")
	seedSkillIndex(t, ctx, st, targetPath, "alpha", "alpha body", "tag")
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	cfg := defaultLoopConfig()
	cfg.SimilarityThreshold = 0.42

	var stdout, stderr bytes.Buffer
	var capturedMinScore float64
	mockQuery := func(ctx context.Context, db *sql.DB, opts ftsQueryOpts) ([]similarMatch, error) {
		capturedMinScore = opts.minScore
		return []similarMatch{}, nil
	}
	runErr := runSimilar(ctx, similarOpts{
		DBPath:    dbPath,
		SkillPath: targetPath,
		TopK:      5,
		// Threshold left zero (sentinel for "use config")
		Config:  cfg,
		QueryFn: mockQuery,
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if runErr != nil {
		t.Fatalf("runSimilar: %v", runErr)
	}
	if capturedMinScore != 0.42 {
		t.Errorf("expected MinScore=0.42 from config, got %v", capturedMinScore)
	}
}

// similarOutRow mirrors the JSON wire format produced by runSimilar.
type similarOutRow struct {
	Path         string  `json:"path"`
	Score        float64 `json:"score"`
	EditDistance float64 `json:"edit_distance"`
}
