package cmd

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

// initStoreForTest stands up an isolated store inside t.TempDir and returns
// its path. The store is closed before returning so the subcommand under
// test can open it without contention.
func initStoreForTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	st, openErr := store.Open(dbPath)
	if openErr != nil {
		t.Fatalf("store.Open: %v", openErr)
	}
	if closeErr := st.Close(); closeErr != nil {
		t.Fatalf("store.Close: %v", closeErr)
	}
	return dbPath
}

// recordFixedNow returns a deterministic clock for record-decision tests.
func recordFixedNow() time.Time { return time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC) }

// TC-UC-35: happy path for action=new-skill — DB row inserted, file created
// with frontmatter containing all 5 required fields and the provided body.
func TestRunRecordDecision_newSkill_createsFileAndRow(t *testing.T) {
	dbPath := initStoreForTest(t)
	workDir := t.TempDir()

	bodyPath := filepath.Join(workDir, "body.txt")
	const bodyContent = "# How to do X\n\nDo X like this.\n"
	if writeErr := os.WriteFile(bodyPath, []byte(bodyContent), 0o644); writeErr != nil {
		t.Fatalf("write body: %v", writeErr)
	}
	targetPath := filepath.Join(workDir, "skills", "do-x", "SKILL.md")

	runErr := runRecordDecision(context.Background(), recordDecisionOpts{
		CandidateSignature: "sig-do-x",
		Action:             "new-skill",
		TargetPath:         targetPath,
		Rationale:          "frequency >= 5",
		BodyFile:           bodyPath,
		Description:        "Do X effectively",
		DBPath:             dbPath,
		Now:                recordFixedNow,
	})
	if runErr != nil {
		t.Fatalf("runRecordDecision: %v", runErr)
	}

	got, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read created skill: %v", readErr)
	}
	gotStr := string(got)
	for _, field := range []string{"name: SKILL", "description: Do X effectively", "learning_provenance:", "- sig-do-x", "2026-05-14T12:00:00Z"} {
		if !strings.Contains(gotStr, field) {
			t.Errorf("expected frontmatter to contain %q, got:\n%s", field, gotStr)
		}
	}
	if !strings.Contains(gotStr, bodyContent) {
		t.Errorf("expected body %q to be embedded; got:\n%s", bodyContent, gotStr)
	}

	st, openErr := store.Open(dbPath)
	if openErr != nil {
		t.Fatalf("store.Open: %v", openErr)
	}
	defer func() { _ = st.Close() }()
	var action, sig string
	scanErr := st.DB().QueryRowContext(context.Background(),
		`SELECT action, candidate_signature FROM decisions WHERE candidate_signature=?`,
		"sig-do-x").Scan(&action, &sig)
	if scanErr != nil {
		t.Fatalf("query decision: %v", scanErr)
	}
	if action != "new-skill" {
		t.Errorf("decision action: want new-skill, got %q", action)
	}
}

// TC-UC-35a: happy path for action=new-memory — file created at the requested
// path with frontmatter; row inserted in `decisions`.
func TestRunRecordDecision_newMemory_createsFileAndRow(t *testing.T) {
	dbPath := initStoreForTest(t)
	workDir := t.TempDir()
	bodyPath := filepath.Join(workDir, "body.txt")
	if writeErr := os.WriteFile(bodyPath, []byte("Prefer X over Y because Z."), 0o644); writeErr != nil {
		t.Fatalf("write body: %v", writeErr)
	}
	targetPath := filepath.Join(workDir, "memory", "prefer-x.md")

	runErr := runRecordDecision(context.Background(), recordDecisionOpts{
		CandidateSignature: "sig-prefer-x",
		Action:             "new-memory",
		TargetPath:         targetPath,
		BodyFile:           bodyPath,
		Description:        "Prefer X",
		DBPath:             dbPath,
		Now:                recordFixedNow,
	})
	if runErr != nil {
		t.Fatalf("runRecordDecision: %v", runErr)
	}
	if _, statErr := os.Stat(targetPath); statErr != nil {
		t.Fatalf("expected file at %s: %v", targetPath, statErr)
	}
}

// TC-UC-35b: business — action=discard inserts the row without touching the
// filesystem (and target-path may be omitted entirely).
func TestRunRecordDecision_discard_rowOnlyNoFile(t *testing.T) {
	dbPath := initStoreForTest(t)
	workDir := t.TempDir()

	runErr := runRecordDecision(context.Background(), recordDecisionOpts{
		CandidateSignature: "sig-discard",
		Action:             "discard",
		Rationale:          "noisy candidate",
		DBPath:             dbPath,
		Now:                recordFixedNow,
	})
	if runErr != nil {
		t.Fatalf("runRecordDecision: %v", runErr)
	}
	entries, readErr := os.ReadDir(workDir)
	if readErr != nil {
		t.Fatalf("readdir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Errorf("expected no file mutation under workDir, got: %v", entries)
	}

	st, openErr := store.Open(dbPath)
	if openErr != nil {
		t.Fatalf("store.Open: %v", openErr)
	}
	defer func() { _ = st.Close() }()
	var action string
	var targetPath sql.NullString
	scanErr := st.DB().QueryRowContext(context.Background(),
		`SELECT action, target_path FROM decisions WHERE candidate_signature='sig-discard'`,
	).Scan(&action, &targetPath)
	if scanErr != nil {
		t.Fatalf("query decision: %v", scanErr)
	}
	if action != "discard" {
		t.Errorf("want action=discard, got %q", action)
	}
	if targetPath.Valid {
		t.Errorf("want NULL target_path, got %q", targetPath.String)
	}
}

// TC-UC-36: action=pending-approval inserts row with the proposed diff and
// does not mutate the target file.
func TestRunRecordDecision_pendingApproval_rowOnly(t *testing.T) {
	dbPath := initStoreForTest(t)
	workDir := t.TempDir()
	diffPath := filepath.Join(workDir, "proposed.diff")
	const diffContent = "+ new line\n- old line\n"
	if writeErr := os.WriteFile(diffPath, []byte(diffContent), 0o644); writeErr != nil {
		t.Fatalf("write diff: %v", writeErr)
	}
	targetPath := filepath.Join(workDir, "skills", "x", "SKILL.md")

	runErr := runRecordDecision(context.Background(), recordDecisionOpts{
		CandidateSignature: "sig-pending",
		Action:             "pending-approval",
		TargetPath:         targetPath,
		DiffFile:           diffPath,
		DBPath:             dbPath,
		Now:                recordFixedNow,
	})
	if runErr != nil {
		t.Fatalf("runRecordDecision: %v", runErr)
	}
	if _, statErr := os.Stat(targetPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("expected target not to be created for pending-approval; statErr=%v", statErr)
	}

	st, openErr := store.Open(dbPath)
	if openErr != nil {
		t.Fatalf("store.Open: %v", openErr)
	}
	defer func() { _ = st.Close() }()
	var action, diff string
	scanErr := st.DB().QueryRowContext(context.Background(),
		`SELECT action, diff FROM decisions WHERE candidate_signature='sig-pending'`,
	).Scan(&action, &diff)
	if scanErr != nil {
		t.Fatalf("query: %v", scanErr)
	}
	if action != "pending-approval" {
		t.Errorf("want action=pending-approval, got %q", action)
	}
	if diff != diffContent {
		t.Errorf("diff mismatch: want %q, got %q", diffContent, diff)
	}
}

// TC-UC-35 (validation): invalid action surfaces as UsageError.
func TestRunRecordDecision_invalidAction_returnsUsageError(t *testing.T) {
	dbPath := initStoreForTest(t)
	runErr := runRecordDecision(context.Background(), recordDecisionOpts{
		CandidateSignature: "sig",
		Action:             "applied", // applied is reserved for apply-decision
		TargetPath:         "x.md",
		DBPath:             dbPath,
	})
	if runErr == nil {
		t.Fatalf("expected UsageError, got nil")
	}
	var ue *learnerr.UsageError
	if !errors.As(runErr, &ue) {
		t.Errorf("want *learnerr.UsageError, got %T: %v", runErr, runErr)
	}
}

// Update path: existing file is updated, frontmatter preserves
// learning_provenance and bumps last_reviewed_at.
func TestRunRecordDecision_update_appendsAndBumpsTimestamp(t *testing.T) {
	dbPath := initStoreForTest(t)
	workDir := t.TempDir()
	target := filepath.Join(workDir, "SKILL.md")
	const existing = "---\nname: SKILL\ndescription: orig\nlearning_provenance:\n  - sig-1\ncreated_at: 2025-01-01T00:00:00Z\nlast_reviewed_at: 2025-01-01T00:00:00Z\n---\nOriginal body.\n"
	if writeErr := os.WriteFile(target, []byte(existing), 0o644); writeErr != nil {
		t.Fatalf("seed: %v", writeErr)
	}
	bodyPath := filepath.Join(workDir, "add.txt")
	if writeErr := os.WriteFile(bodyPath, []byte("Appended block."), 0o644); writeErr != nil {
		t.Fatalf("body: %v", writeErr)
	}

	runErr := runRecordDecision(context.Background(), recordDecisionOpts{
		CandidateSignature: "sig-2",
		Action:             "update",
		TargetPath:         target,
		BodyFile:           bodyPath,
		Mode:               "append",
		DBPath:             dbPath,
		Now:                recordFixedNow,
	})
	if runErr != nil {
		t.Fatalf("update: %v", runErr)
	}
	got, _ := os.ReadFile(target)
	gotStr := string(got)
	if !strings.Contains(gotStr, `last_reviewed_at: "2026-05-14T12:00:00Z"`) {
		t.Errorf("expected bumped last_reviewed_at, got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "Original body.") || !strings.Contains(gotStr, "Appended block.") {
		t.Errorf("expected both bodies present, got:\n%s", gotStr)
	}
}
