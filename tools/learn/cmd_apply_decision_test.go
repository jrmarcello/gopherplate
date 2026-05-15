package learn

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// applyFixedNow returns a deterministic timestamp for apply-decision tests.
func applyFixedNow() time.Time { return time.Date(2026, 5, 14, 9, 30, 0, 0, time.UTC) }

// seedPendingDecision inserts a `pending-approval` row that targets path, and
// returns the row id. Used by the apply-decision tests below.
func seedPendingDecision(t *testing.T, dbPath, target, diff string) int64 {
	t.Helper()
	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	defer func() { _ = st.Close() }()

	res, execErr := st.DB().ExecContext(context.Background(),
		`INSERT INTO decisions (candidate_signature, action, target_path, rationale, diff, decided_at)
		 VALUES (?, 'pending-approval', ?, '', ?, ?)`,
		"sig-apply", target, diff, applyFixedNow().Format(time.RFC3339),
	)
	if execErr != nil {
		t.Fatalf("seed decision: %v", execErr)
	}
	id, idErr := res.LastInsertId()
	if idErr != nil {
		t.Fatalf("last insert id: %v", idErr)
	}
	return id
}

// TC-UC-36a: apply-decision moves the target file under _deprecated/<base>-<TS>.md
// with a literal header, and bumps the row action to 'applied'.
func TestRunApplyDecision_movesFileAndMarksApplied(t *testing.T) {
	dbPath := initStoreForTest(t)
	workDir := t.TempDir()
	target := filepath.Join(workDir, "skills", "x", "SKILL.md")
	if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	const original = "original SKILL body\n"
	if writeErr := os.WriteFile(target, []byte(original), 0o644); writeErr != nil {
		t.Fatalf("seed: %v", writeErr)
	}

	id := seedPendingDecision(t, dbPath, target, "+ some diff\n")

	runErr := runApplyDecision(context.Background(), applyDecisionOpts{
		ID:     id,
		DBPath: dbPath,
		Now:    applyFixedNow,
	})
	if runErr != nil {
		t.Fatalf("runApplyDecision: %v", runErr)
	}

	// Original gone.
	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("expected original file removed; statErr=%v", statErr)
	}

	// Deprecated file present with expected name + header.
	wantPath := filepath.Join(filepath.Dir(target), "_deprecated", "SKILL-20260514T093000Z.md")
	got, readErr := os.ReadFile(wantPath)
	if readErr != nil {
		t.Fatalf("read deprecated file %q: %v", wantPath, readErr)
	}
	gotStr := string(got)
	if !strings.HasPrefix(gotStr, "> Deprecated 2026-05-14 by /learn-refine: merged into ") {
		t.Errorf("expected leading header; got first line: %q", strings.SplitN(gotStr, "\n", 2)[0])
	}
	if !strings.Contains(gotStr, original) {
		t.Errorf("expected original body preserved; got:\n%s", gotStr)
	}

	// Row updated.
	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	defer func() { _ = st.Close() }()
	var action string
	scanErr := st.DB().QueryRowContext(context.Background(),
		`SELECT action FROM decisions WHERE id=?`, id).Scan(&action)
	if scanErr != nil {
		t.Fatalf("query: %v", scanErr)
	}
	if action != "applied" {
		t.Errorf("want action=applied, got %q", action)
	}
}

// TC-UC-36b (edge): --dry-run prints the diff to stdout and leaves the file
// + row untouched.
func TestRunApplyDecision_dryRun_printsDiffAndDoesNotMutate(t *testing.T) {
	dbPath := initStoreForTest(t)
	workDir := t.TempDir()
	target := filepath.Join(workDir, "x.md")
	if writeErr := os.WriteFile(target, []byte("body\n"), 0o644); writeErr != nil {
		t.Fatalf("seed: %v", writeErr)
	}
	const diff = "+ foo\n- bar\n"
	id := seedPendingDecision(t, dbPath, target, diff)

	var buf bytes.Buffer
	runErr := runApplyDecision(context.Background(), applyDecisionOpts{
		ID:     id,
		DBPath: dbPath,
		DryRun: true,
		Stdout: &buf,
		Now:    applyFixedNow,
	})
	if runErr != nil {
		t.Fatalf("dry-run: %v", runErr)
	}
	if buf.String() != diff {
		t.Errorf("want stdout=%q, got %q", diff, buf.String())
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("expected file untouched, got statErr=%v", statErr)
	}

	st, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("openStore: %v", openErr)
	}
	defer func() { _ = st.Close() }()
	var action string
	scanErr := st.DB().QueryRowContext(context.Background(),
		`SELECT action FROM decisions WHERE id=?`, id).Scan(&action)
	if scanErr != nil {
		t.Fatalf("query: %v", scanErr)
	}
	if action != "pending-approval" {
		t.Errorf("want action unchanged (pending-approval), got %q", action)
	}
}

// Validation: a non-existent id surfaces as UsageError (typed assertion via
// errors.As — string-contains alone would let a mutant return *RuntimeError
// with the same message and break the exit-code contract).
func TestRunApplyDecision_noSuchID_returnsUsageError(t *testing.T) {
	dbPath := initStoreForTest(t)
	runErr := runApplyDecision(context.Background(), applyDecisionOpts{
		ID: 9999, DBPath: dbPath,
	})
	if runErr == nil {
		t.Fatalf("expected error, got nil")
	}
	var usageErr *usageError
	if !errors.As(runErr, &usageErr) {
		t.Errorf("expected *usageError, got %T: %v", runErr, runErr)
	}
	if !strings.Contains(runErr.Error(), "no decision row") {
		t.Errorf("expected 'no decision row' in error message; got %v", runErr)
	}
}
