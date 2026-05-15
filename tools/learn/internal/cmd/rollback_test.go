package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/audit"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
)

// Builds a minimal learning dir + applied decision, then exercises rollback.
// The fixture writes one canonical SKILL.md, runs refine-apply to deprecate it,
// then asserts rollback restores it byte-for-byte.
func TestRunRollback_restoresFileAndAppendsAuditEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := initStoreForTest(t)
	canonical := filepath.Join(dir, "skills", "demo", "SKILL.md")
	if mkErr := os.MkdirAll(filepath.Dir(canonical), 0o750); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	const originalBody = "demo body line 1\ndemo body line 2\n"
	if writeErr := os.WriteFile(canonical, []byte(originalBody), 0o600); writeErr != nil {
		t.Fatalf("seed canonical: %v", writeErr)
	}

	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	if applyErr := runRefineApply(context.Background(), refineApplyOpts{
		CandidateSignature: "merge:demo+other",
		TargetPath:         canonical,
		MergedInto:         filepath.Join(dir, "skills", "demo-canonical", "SKILL.md"),
		Rationale:          "near-duplicate",
		DBPath:             dbPath,
		Stdout:             os.Stdout,
		Now:                func() time.Time { return now },
	}); applyErr != nil {
		t.Fatalf("runRefineApply: %v", applyErr)
	}

	// Locate the decision id via the audit entry (only one so far).
	entries, readErr := audit.Read(audit.LearningDirFromDB(dbPath))
	if readErr != nil {
		t.Fatalf("audit.Read: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 audit entry, got %d", len(entries))
	}
	id := entries[0].DecisionID

	// Roll it back. Now bump 1 minute so the rolled-back row's timestamp differs.
	later := now.Add(1 * time.Minute)
	if rbErr := runRollback(context.Background(), rollbackOpts{
		ID:     id,
		DBPath: dbPath,
		Stdout: os.Stdout,
		Now:    func() time.Time { return later },
	}); rbErr != nil {
		t.Fatalf("runRollback: %v", rbErr)
	}

	// Canonical restored byte-for-byte (header stripped).
	got, readErr := os.ReadFile(canonical)
	if readErr != nil {
		t.Fatalf("read restored: %v", readErr)
	}
	if string(got) != originalBody {
		t.Errorf("restored body mismatch:\nwant=%q\n got=%q", originalBody, got)
	}

	// Deprecated copy gone.
	if _, statErr := os.Stat(entries[0].DeprecatedPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("expected deprecated file removed; statErr=%v", statErr)
	}

	// Two audit entries now: applied + rolled-back, same DecisionID.
	final, _ := audit.Read(audit.LearningDirFromDB(dbPath))
	if len(final) != 2 {
		t.Fatalf("want 2 audit entries (applied + rolled-back), got %d", len(final))
	}
	if final[1].Action != "rolled-back" {
		t.Errorf("want second entry action=rolled-back, got %q", final[1].Action)
	}
	if final[1].DecisionID != id {
		t.Errorf("rollback entry should share DecisionID=%d, got %d", id, final[1].DecisionID)
	}
}

func TestRunRollback_noAuditEntryReturnsUsageError(t *testing.T) {
	t.Parallel()
	dbPath := initStoreForTest(t)
	err := runRollback(context.Background(), rollbackOpts{
		ID: 9999, DBPath: dbPath, Stdout: os.Stdout,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var ue *learnerr.UsageError
	if !errors.As(err, &ue) {
		t.Errorf("want UsageError, got %T", err)
	}
}

func TestRunRollback_alreadyRolledBackIsUsageError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	learningDir := dir
	// Seed two audit entries: applied + rolled-back for the same id.
	if appendErr := audit.Append(learningDir, audit.Entry{
		DecisionID:     1,
		Action:         "applied",
		SourcePath:     filepath.Join(dir, "x.md"),
		DeprecatedPath: filepath.Join(dir, "_deprecated", "x.md"),
	}); appendErr != nil {
		t.Fatalf("seed applied: %v", appendErr)
	}
	if appendErr := audit.Append(learningDir, audit.Entry{
		DecisionID:     1,
		Action:         "rolled-back",
		SourcePath:     filepath.Join(dir, "x.md"),
		DeprecatedPath: filepath.Join(dir, "_deprecated", "x.md"),
	}); appendErr != nil {
		t.Fatalf("seed rolled-back: %v", appendErr)
	}
	dbPath := filepath.Join(dir, "db.sqlite")
	if err := runRollback(context.Background(), rollbackOpts{
		ID: 1, DBPath: dbPath, Stdout: os.Stdout,
	}); err == nil || !strings.Contains(err.Error(), "already rolled back") {
		t.Errorf("want already-rolled-back error, got %v", err)
	}
}

func TestStripDeprecationHeader_removesHeaderAndBlank(t *testing.T) {
	t.Parallel()
	in := []byte("> Deprecated 2026-05-14 by /learn-refine: merged into x.md\n\nbody line 1\nbody line 2\n")
	got := stripDeprecationHeader(in)
	const want = "body line 1\nbody line 2\n"
	if string(got) != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestStripDeprecationHeader_leavesUnknownInputAlone(t *testing.T) {
	t.Parallel()
	in := []byte("no header here\nplain text\n")
	got := stripDeprecationHeader(in)
	if string(got) != string(in) {
		t.Errorf("want unchanged %q, got %q", in, got)
	}
}
