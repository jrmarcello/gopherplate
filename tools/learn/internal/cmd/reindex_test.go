package cmd

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

// openStoreForTest is a tiny test-only wrapper around store.Open so failure
// surfaces with t.Fatalf-friendly types in callers.
func openStoreForTest(t *testing.T, dbPath string) (*store.Store, error) {
	t.Helper()
	return store.Open(dbPath)
}

// setupTempLearningDir creates a temp dir tree:
//
//	<root>/learning/db.sqlite (created by runInit)
//	<root>/learning/config.yml (created by runInit)
//	<root>/skills/<slug>/SKILL.md
//	<root>/memory/<name>.md
//
// It returns the root path along with the resolved dbPath, skillsDir, and memoryDir.
// root is part of the contract for future callers that need to walk siblings
// of the learning dir (e.g. wiring fragment tests). Linters flag it as unused
// by current callers, but exposing it keeps the helper extensible without
// reshuffling 9 call sites.
//
//nolint:unparam // root retained for callers that may need the temp root
func setupTempLearningDir(t *testing.T) (root, dbPath, skillsDir, memoryDir, configPath string) {
	t.Helper()
	root = t.TempDir()
	learningDir := filepath.Join(root, "learning")
	if runErr := runInit(initOpts{LearningDir: learningDir}); runErr != nil {
		t.Fatalf("setup: runInit: %v", runErr)
	}
	dbPath = filepath.Join(learningDir, "db.sqlite")
	configPath = filepath.Join(learningDir, "config.yml")
	skillsDir = filepath.Join(root, "skills")
	memoryDir = filepath.Join(root, "memory")
	if mkErr := os.MkdirAll(skillsDir, 0o755); mkErr != nil {
		t.Fatalf("setup: mkdir skills: %v", mkErr)
	}
	if mkErr := os.MkdirAll(memoryDir, 0o755); mkErr != nil {
		t.Fatalf("setup: mkdir memory: %v", mkErr)
	}
	return root, dbPath, skillsDir, memoryDir, configPath
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
		t.Fatalf("mkdir for %q: %v", path, mkErr)
	}
	if writeErr := os.WriteFile(path, []byte(content), 0o644); writeErr != nil {
		t.Fatalf("write %q: %v", path, writeErr)
	}
}

const skillSample1 = `---
name: sample-skill
description: A sample skill for testing.
tags: [test, sample]
---

# Sample Skill

Body content goes here.
`

const skillSample2 = `---
name: another-skill
description: Another skill.
---

# Another Skill

More body.
`

const memorySample1 = `---
name: sample-memory
description: A memory entry.
metadata:
  type: feedback
---

Body of memory entry.
`

func countRows(t *testing.T, dbPath, table string) int {
	t.Helper()
	st, openErr := openStoreForTest(t, dbPath)
	if openErr != nil {
		t.Fatalf("countRows: open: %v", openErr)
	}
	defer func() { _ = st.Close() }()
	var n int
	if scanErr := st.DB().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM "+table).Scan(&n); scanErr != nil {
		t.Fatalf("countRows %s: %v", table, scanErr)
	}
	return n
}

// TC-UC-12: full reindex of a temp tree populates skill_index + memory_index.
func TestRunReindex_happy(t *testing.T) {
	_, dbPath, skillsDir, memoryDir, configPath := setupTempLearningDir(t)

	writeFile(t, filepath.Join(skillsDir, "sample-skill", "SKILL.md"), skillSample1)
	writeFile(t, filepath.Join(skillsDir, "another-skill", "SKILL.md"), skillSample2)
	writeFile(t, filepath.Join(memoryDir, "sample-memory.md"), memorySample1)

	if runErr := runReindex(context.Background(), reindexOpts{
		DBPath:     dbPath,
		SkillsDir:  skillsDir,
		MemoryDir:  memoryDir,
		ConfigPath: configPath,
	}); runErr != nil {
		t.Fatalf("runReindex: %v", runErr)
	}

	if got, want := countRows(t, dbPath, "skill_index"), 2; got != want {
		t.Errorf("skill_index count: got %d, want %d", got, want)
	}
	if got, want := countRows(t, dbPath, "memory_index"), 1; got != want {
		t.Errorf("memory_index count: got %d, want %d", got, want)
	}
}

// TC-UC-12a: running reindex twice produces identical row counts (idempotent).
func TestRunReindex_idempotent(t *testing.T) {
	_, dbPath, skillsDir, memoryDir, configPath := setupTempLearningDir(t)

	writeFile(t, filepath.Join(skillsDir, "sample-skill", "SKILL.md"), skillSample1)
	writeFile(t, filepath.Join(memoryDir, "sample-memory.md"), memorySample1)

	opts := reindexOpts{
		DBPath:     dbPath,
		SkillsDir:  skillsDir,
		MemoryDir:  memoryDir,
		ConfigPath: configPath,
	}
	if runErr := runReindex(context.Background(), opts); runErr != nil {
		t.Fatalf("first runReindex: %v", runErr)
	}
	skillCount1 := countRows(t, dbPath, "skill_index")
	memCount1 := countRows(t, dbPath, "memory_index")

	if runErr := runReindex(context.Background(), opts); runErr != nil {
		t.Fatalf("second runReindex: %v", runErr)
	}
	if got := countRows(t, dbPath, "skill_index"); got != skillCount1 {
		t.Errorf("skill_index count drifted: got %d, want %d", got, skillCount1)
	}
	if got := countRows(t, dbPath, "memory_index"); got != memCount1 {
		t.Errorf("memory_index count drifted: got %d, want %d", got, memCount1)
	}
}

// TC-UC-13: deleting a skill file → reindex removes the orphan row.
func TestRunReindex_orphanRemoved(t *testing.T) {
	_, dbPath, skillsDir, memoryDir, configPath := setupTempLearningDir(t)

	skillPath := filepath.Join(skillsDir, "sample-skill", "SKILL.md")
	writeFile(t, skillPath, skillSample1)
	writeFile(t, filepath.Join(skillsDir, "another-skill", "SKILL.md"), skillSample2)

	opts := reindexOpts{
		DBPath:     dbPath,
		SkillsDir:  skillsDir,
		MemoryDir:  memoryDir,
		ConfigPath: configPath,
	}
	if runErr := runReindex(context.Background(), opts); runErr != nil {
		t.Fatalf("runReindex: %v", runErr)
	}
	if got, want := countRows(t, dbPath, "skill_index"), 2; got != want {
		t.Fatalf("pre-orphan skill count: got %d, want %d", got, want)
	}

	// Delete one skill file on disk.
	if rmErr := os.RemoveAll(filepath.Join(skillsDir, "sample-skill")); rmErr != nil {
		t.Fatalf("rm sample-skill: %v", rmErr)
	}

	if runErr := runReindex(context.Background(), opts); runErr != nil {
		t.Fatalf("runReindex after delete: %v", runErr)
	}
	if got, want := countRows(t, dbPath, "skill_index"), 1; got != want {
		t.Errorf("orphan not removed: got %d, want %d", got, want)
	}
}

// TC-UC-19-sinceMtime (extra coverage of REQ-19): --since-mtime filters out
// files older than the window. A mutant that ignores SinceMtime and always
// does a full reindex would update both files; this test pins the filter
// semantics.
func TestRunReindex_sinceMtime_onlyUpdatesRecent(t *testing.T) {
	_, dbPath, skillsDir, memoryDir, configPath := setupTempLearningDir(t)

	skillNew := filepath.Join(skillsDir, "new-skill", "SKILL.md")
	skillOld := filepath.Join(skillsDir, "old-skill", "SKILL.md")
	writeFile(t, skillNew, skillSample1)
	writeFile(t, skillOld, skillSample2)

	// Push skillOld's mtime well into the past (10 days). skillNew keeps the
	// current mtime that the OS just set.
	old := time.Now().Add(-10 * 24 * time.Hour)
	if chErr := os.Chtimes(skillOld, old, old); chErr != nil {
		t.Fatalf("chtimes: %v", chErr)
	}

	opts := reindexOpts{
		DBPath:     dbPath,
		SkillsDir:  skillsDir,
		MemoryDir:  memoryDir,
		ConfigPath: configPath,
		SinceMtime: 1 * time.Hour, // only recent files
	}
	if runErr := runReindex(context.Background(), opts); runErr != nil {
		t.Fatalf("runReindex: %v", runErr)
	}

	st, openErr := openStoreForTest(t, dbPath)
	if openErr != nil {
		t.Fatalf("open: %v", openErr)
	}
	defer func() { _ = st.Close() }()

	// skillNew indexed; skillOld absent from index.
	var newCount, oldCount int
	if scanErr := st.DB().QueryRowContext(context.Background(),
		`SELECT count(*) FROM skill_index WHERE path=?`, skillNew).Scan(&newCount); scanErr != nil {
		t.Fatalf("query new: %v", scanErr)
	}
	if scanErr := st.DB().QueryRowContext(context.Background(),
		`SELECT count(*) FROM skill_index WHERE path=?`, skillOld).Scan(&oldCount); scanErr != nil {
		t.Fatalf("query old: %v", scanErr)
	}
	if newCount != 1 {
		t.Errorf("expected new skill indexed (count=1), got %d", newCount)
	}
	if oldCount != 0 {
		t.Errorf("expected old skill skipped by --since-mtime (count=0), got %d", oldCount)
	}
}

// TC-UC-14: --path <single-file> only updates that one entry.
//
// Uses injected Now so the two reindex passes get distinct timestamps
// deterministically — no time.Sleep race in CI.
func TestRunReindex_singlePath_partialUpdate(t *testing.T) {
	_, dbPath, skillsDir, memoryDir, configPath := setupTempLearningDir(t)

	skillA := filepath.Join(skillsDir, "sample-skill", "SKILL.md")
	skillB := filepath.Join(skillsDir, "another-skill", "SKILL.md")
	writeFile(t, skillA, skillSample1)
	writeFile(t, skillB, skillSample2)

	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(1 * time.Minute) // distinct timestamp for the second pass.

	opts := reindexOpts{
		DBPath:     dbPath,
		SkillsDir:  skillsDir,
		MemoryDir:  memoryDir,
		ConfigPath: configPath,
		Now:        func() time.Time { return t0 },
	}
	if runErr := runReindex(context.Background(), opts); runErr != nil {
		t.Fatalf("initial runReindex: %v", runErr)
	}

	// Capture indexed_at of skillB.
	st, openErr := openStoreForTest(t, dbPath)
	if openErr != nil {
		t.Fatalf("open store: %v", openErr)
	}
	var beforeIndexedAt string
	if scanErr := st.DB().QueryRowContext(context.Background(),
		`SELECT indexed_at FROM skill_index WHERE path=?`, skillB).Scan(&beforeIndexedAt); scanErr != nil {
		t.Fatalf("read skillB indexed_at: %v", scanErr)
	}
	_ = st.Close()

	// Update skillA on disk and reindex only it with a clock advanced by 1
	// minute. No time.Sleep needed — the injected Now guarantees a
	// strictly-greater RFC3339 timestamp on skillA's row.
	writeFile(t, skillA, skillSample1+"\nextra content\n")
	singleOpts := opts
	singleOpts.SinglePath = skillA
	singleOpts.Now = func() time.Time { return t1 }
	if runErr := runReindex(context.Background(), singleOpts); runErr != nil {
		t.Fatalf("single-path runReindex: %v", runErr)
	}

	// skillB indexed_at unchanged.
	st2, openErr2 := openStoreForTest(t, dbPath)
	if openErr2 != nil {
		t.Fatalf("open store 2: %v", openErr2)
	}
	defer func() { _ = st2.Close() }()
	var afterIndexedAt string
	if scanErr := st2.DB().QueryRowContext(context.Background(),
		`SELECT indexed_at FROM skill_index WHERE path=?`, skillB).Scan(&afterIndexedAt); scanErr != nil {
		t.Fatalf("read skillB indexed_at after: %v", scanErr)
	}
	if afterIndexedAt != beforeIndexedAt {
		t.Errorf("skillB indexed_at changed: before=%q after=%q (single-path should not touch other rows)",
			beforeIndexedAt, afterIndexedAt)
	}

	// skillA body should contain the new content marker.
	var body string
	if scanErr := st2.DB().QueryRowContext(context.Background(),
		`SELECT body FROM skill_index WHERE path=?`, skillA).Scan(&body); scanErr != nil {
		t.Fatalf("read skillA body: %v", scanErr)
	}
	if !strings.Contains(body, "extra content") {
		t.Errorf("skillA body not updated: %q", body)
	}
}

// TC-UC-15: a file directly inside <dir>/_deprecated/ with no prior entry
// must not be indexed.
func TestRunReindex_deprecatedNotIndexed(t *testing.T) {
	_, dbPath, skillsDir, memoryDir, configPath := setupTempLearningDir(t)

	// Skill file under _deprecated.
	writeFile(t, filepath.Join(skillsDir, "_deprecated", "old-skill", "SKILL.md"), skillSample1)
	// Memory file under _deprecated.
	writeFile(t, filepath.Join(memoryDir, "_deprecated", "old-memory.md"), memorySample1)

	if runErr := runReindex(context.Background(), reindexOpts{
		DBPath:     dbPath,
		SkillsDir:  skillsDir,
		MemoryDir:  memoryDir,
		ConfigPath: configPath,
	}); runErr != nil {
		t.Fatalf("runReindex: %v", runErr)
	}

	if got := countRows(t, dbPath, "skill_index"); got != 0 {
		t.Errorf("deprecated skill must not be indexed: got %d rows", got)
	}
	if got := countRows(t, dbPath, "memory_index"); got != 0 {
		t.Errorf("deprecated memory must not be indexed: got %d rows", got)
	}
}

// TC-UC-15a: prior row for <dir>/foo/SKILL.md whose file moved to
// <dir>/_deprecated/foo.md (or <dir>/_deprecated/foo/SKILL.md) — the original
// path is gone but the deprecated counterpart exists; the row is preserved
// (not deleted as orphan).
func TestRunReindex_deprecatedCounterpartPreservesRow(t *testing.T) {
	_, dbPath, skillsDir, memoryDir, configPath := setupTempLearningDir(t)

	skillPath := filepath.Join(skillsDir, "sample-skill", "SKILL.md")
	writeFile(t, skillPath, skillSample1)

	opts := reindexOpts{
		DBPath:     dbPath,
		SkillsDir:  skillsDir,
		MemoryDir:  memoryDir,
		ConfigPath: configPath,
	}
	if runErr := runReindex(context.Background(), opts); runErr != nil {
		t.Fatalf("initial runReindex: %v", runErr)
	}
	if got := countRows(t, dbPath, "skill_index"); got != 1 {
		t.Fatalf("pre-move skill count: got %d, want 1", got)
	}

	// "Move" the skill into _deprecated (we remove the original and create the
	// deprecated counterpart at the conventional location).
	if rmErr := os.RemoveAll(filepath.Join(skillsDir, "sample-skill")); rmErr != nil {
		t.Fatalf("rm original: %v", rmErr)
	}
	writeFile(t, filepath.Join(skillsDir, "_deprecated", "sample-skill.md"), skillSample1)

	if runErr := runReindex(context.Background(), opts); runErr != nil {
		t.Fatalf("post-move runReindex: %v", runErr)
	}
	if got := countRows(t, dbPath, "skill_index"); got != 1 {
		t.Errorf("row should be preserved (deprecated counterpart exists): got %d, want 1", got)
	}
}

// TC-UC-15b: a memory file containing a wikilink that resolves under
// <dir>/_deprecated/ emits a structured slog warn.
func TestRunReindex_brokenLinkEmitsWarn(t *testing.T) {
	_, dbPath, skillsDir, memoryDir, configPath := setupTempLearningDir(t)

	// Create the deprecated target so the broken-link audit triggers.
	writeFile(t, filepath.Join(memoryDir, "_deprecated", "deprecated-link.md"), memorySample1)
	// Memory file with a wikilink to the deprecated target.
	memBody := `---
name: source-memory
description: Source.
metadata:
  type: feedback
---

This points to [[deprecated-link]] which is retired.
`
	writeFile(t, filepath.Join(memoryDir, "source-memory.md"), memBody)

	// Capture slog output.
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	if runErr := runReindex(context.Background(), reindexOpts{
		DBPath:     dbPath,
		SkillsDir:  skillsDir,
		MemoryDir:  memoryDir,
		ConfigPath: configPath,
		Stderr:     &bytes.Buffer{},
	}); runErr != nil {
		t.Fatalf("runReindex: %v", runErr)
	}

	out := buf.String()
	if !strings.Contains(out, "broken_link") {
		t.Errorf("expected slog warn with msg=broken_link, got: %q", out)
	}
	if !strings.Contains(out, "deprecated-link") {
		t.Errorf("expected target=deprecated-link in slog output: %q", out)
	}
}
