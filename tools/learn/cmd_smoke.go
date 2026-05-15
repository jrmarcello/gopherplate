package learn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// smoke wires the learning-loop pipeline end-to-end against synthetic fixtures
// in a tempdir. Verifies REQ-28 / TC-E2E-06. Touches no real project state.
//
// Stages exercised:
//
//	init → complete-task ×3 → extract → reindex → recall → stats
//
// On success, prints "SMOKE OK" plus the captured stats JSON to stdout.

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newSmokeCmd())
	})
}

func newSmokeCmd() *cobra.Command {
	var keepArtifacts bool
	c := &cobra.Command{
		Use:   "smoke",
		Short: "End-to-end fixture smoke test of the learning loop",
		Long: `smoke initializes a temporary store, ingests fixture specs, runs extract,
reindex and recall, then prints stats. Used by 'make learn-smoke' / TC-E2E-06.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSmoke(cmd.Context(), smokeOpts{
				KeepArtifacts: keepArtifacts,
				Stdout:        cmd.OutOrStdout(),
				Stderr:        cmd.ErrOrStderr(),
				Now:           time.Now,
			})
		},
	}
	c.Flags().BoolVar(&keepArtifacts, "keep", false,
		"Keep the temp learning dir after the smoke (default: remove)")
	return c
}

type smokeOpts struct {
	KeepArtifacts bool
	Stdout        io.Writer
	Stderr        io.Writer
	Now           func() time.Time
}

func runSmoke(ctx context.Context, o smokeOpts) error {
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.Now == nil {
		o.Now = time.Now
	}

	dir, mkErr := os.MkdirTemp("", "learn-smoke-")
	if mkErr != nil {
		return runtimef("smoke: mkdir temp: %w", mkErr)
	}
	if !o.KeepArtifacts {
		defer func() { _ = os.RemoveAll(dir) }()
	}

	// 1. init — writes config.yml + db.sqlite under <dir>/.claude/learning/.
	learningDir := filepath.Join(dir, ".claude", "learning")
	if initErr := runInit(initOpts{
		LearningDir: learningDir,
	}); initErr != nil {
		return runtimef("smoke: init: %w", initErr)
	}
	dbPath := filepath.Join(learningDir, "db.sqlite")
	configPath := filepath.Join(learningDir, "config.yml")

	// 2. seed 3 fake specs + verify complete-task increments counter.
	specsDir := filepath.Join(dir, "specs")
	if mkErr := os.MkdirAll(specsDir, 0o750); mkErr != nil {
		return runtimef("smoke: mkdir specs: %w", mkErr)
	}
	for i := 1; i <= 3; i++ {
		specPath := filepath.Join(specsDir, fmt.Sprintf("smoke-%d.md", i))
		body := fmt.Sprintf(`# Spec: smoke-%d

## Status: DONE

## Tasks

- [x] **TASK-1**: Read foo.go, edit, run tests
  - files: %s/foo.go
- [x] **TASK-2**: Read bar.go, edit, run tests
  - files: %s/bar.go

## Execution Log

### TASK-1 (2026-05-14 10:0%d)
TDD: RED(2) → GREEN(2) → REFACTOR(clean)

### TASK-2 (2026-05-14 10:1%d)
TDD: RED(2) → GREEN(2) → REFACTOR(clean)
`, i, specsDir, specsDir, i, i)
		if writeErr := os.WriteFile(specPath, []byte(body), 0o600); writeErr != nil {
			return runtimef("smoke: write spec %d: %w", i, writeErr)
		}
		// Call complete-task; uses default config (nudge_threshold=5 → won't fire).
		ctOpts := completeOpts{
			DBPath:     dbPath,
			ConfigPath: configPath,
			SpecPath:   specPath,
			SessionID:  fmt.Sprintf("smoke-session-%d", i),
			Stderr:     o.Stderr,
			Now:        o.Now,
		}
		if ctErr := runCompleteTask(ctx, ctOpts); ctErr != nil {
			return runtimef("smoke: complete-task %d: %w", i, ctErr)
		}
	}

	// Verify events table has 3 rows.
	st, openErr := openStore(dbPath)
	if openErr != nil {
		return runtimef("smoke: reopen store: %w", openErr)
	}
	defer func() { _ = st.Close() }()
	var eventsCount int
	if countErr := st.DB().QueryRowContext(ctx, "SELECT count(*) FROM events").Scan(&eventsCount); countErr != nil {
		return runtimef("smoke: count events: %w", countErr)
	}
	if eventsCount != 3 {
		return runtimef("smoke: expected 3 events, got %d", eventsCount)
	}
	if closeErr := st.Close(); closeErr != nil {
		return runtimef("smoke: close store before extract: %w", closeErr)
	}

	// 3. extract — write candidates.jsonl. Lower MIN_PATTERN_FREQ to 2 so 3-spec
	// fixture exercises the path even with limited repetition.
	cfg, loadErr := loadConfig(configPath)
	if loadErr != nil {
		return runtimef("smoke: load config: %w", loadErr)
	}
	candidatesPath := filepath.Join(dir, "candidates.jsonl")
	extOpts := extractOpts{
		DBPath:          dbPath,
		ConfigPath:      configPath,
		SpecsDir:        specsDir,
		Out:             candidatesPath,
		MinFreqOverride: 2,
		Config:          cfg,
		Stderr:          o.Stderr,
		Now:             o.Now,
	}
	if extErr := runExtract(ctx, extOpts); extErr != nil {
		return runtimef("smoke: extract: %w", extErr)
	}
	// candidates.jsonl must exist (may be empty if no n-grams reach freq>=2,
	// which is fine — we assert the file).
	if _, statErr := os.Stat(candidatesPath); statErr != nil {
		return runtimef("smoke: candidates.jsonl missing: %w", statErr)
	}

	// 4. reindex — seed a fake skill, then reindex.
	skillDir := filepath.Join(dir, "skills", "smoke-skill")
	if mkErr := os.MkdirAll(skillDir, 0o750); mkErr != nil {
		return runtimef("smoke: mkdir skill: %w", mkErr)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	skillBody := `---
name: smoke-skill
description: Smoke test skill for the learning loop
learning_provenance: ["smoke:fixture"]
created_at: 2026-05-14T10:00:00Z
last_reviewed_at: 2026-05-14T10:00:00Z
---

# /smoke-skill

This skill exists only inside the smoke test temp dir. It contains the
keyword refactor-user-cache used by recall to verify the retrieval pipeline.
`
	if writeErr := os.WriteFile(skillPath, []byte(skillBody), 0o600); writeErr != nil {
		return runtimef("smoke: write skill: %w", writeErr)
	}
	rxOpts := reindexOpts{
		DBPath:    dbPath,
		SkillsDir: filepath.Join(dir, "skills"),
		MemoryDir: "", // none — empty dir; reindex tolerates missing
		Stderr:    o.Stderr,
		Now:       o.Now,
	}
	if rxErr := runReindex(ctx, rxOpts); rxErr != nil {
		return runtimef("smoke: reindex: %w", rxErr)
	}

	// 5. recall — query for the seeded skill's keyword. Force min-score low so
	// FTS5 score variability doesn't flake.
	var recallBuf strings.Builder
	rcOpts := recallOpts{
		DBPath:    dbPath,
		Prompt:    "refactor-user-cache pipeline retrieval",
		TopK:      3,
		MaxTokens: 500,
		MinScore:  0.0,
		Format:    "json",
		Config:    cfg,
		Stdout:    &recallBuf,
		Stderr:    o.Stderr,
	}
	if rcErr := runRecall(ctx, rcOpts); rcErr != nil {
		return runtimef("smoke: recall: %w", rcErr)
	}
	if !strings.Contains(recallBuf.String(), "smoke-skill") {
		return runtimef("smoke: recall output missing seeded skill — got: %s",
			truncateForError(recallBuf.String(), 200))
	}

	// 6. stats — final check that the JSON output is well-formed and reflects
	// our seeded state.
	var statsBuf strings.Builder
	stOpts := statsOpts{
		DBPath: dbPath,
		Stdout: &statsBuf,
		Stderr: o.Stderr,
	}
	if stErr := runStats(ctx, stOpts); stErr != nil {
		return runtimef("smoke: stats: %w", stErr)
	}
	var statsPayload map[string]any
	if jsonErr := json.Unmarshal([]byte(statsBuf.String()), &statsPayload); jsonErr != nil {
		return runtimef("smoke: stats output not JSON: %w", jsonErr)
	}

	_, _ = fmt.Fprintln(o.Stdout, "SMOKE OK")
	_, _ = fmt.Fprintln(o.Stdout, statsBuf.String())
	return nil
}

func truncateForError(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
