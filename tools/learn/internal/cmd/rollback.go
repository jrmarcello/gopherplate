// rollback subcommand (REQ-9a follow-up). Restores a previously-deprecated
// file from .claude/skills/_deprecated/ (or memory/_deprecated/) back to its
// canonical path by consulting audit.jsonl for the move details. Inserts a
// new `decisions` row with action='rolled-back' and appends a matching
// audit.jsonl entry so the trail is symmetric.
//
// This subcommand is what makes the auto-apply policy operationally safe:
// the agent applies merges autonomously, but every move is reversible in
// one shot without manual `mv` gymnastics.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/audit"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newRollbackCmd())
	})
}

func newRollbackCmd() *cobra.Command {
	var (
		id     int64
		dbPath string
		dryRun bool
	)
	c := &cobra.Command{
		Use:   "rollback",
		Short: "Restore a deprecated file back to its canonical path (reverses apply-decision)",
		Long: `rollback looks up the most-recent audit.jsonl entry for the given decision id,
strips the deprecation header from the file under _deprecated/, and moves it
back to its original path. A new decisions row with action='rolled-back' is
inserted and a matching audit.jsonl entry is appended.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRollback(cmd.Context(), rollbackOpts{
				ID:     id,
				DBPath: dbPath,
				DryRun: dryRun,
				Stdout: cmd.OutOrStdout(),
			})
		},
	}
	c.Flags().Int64Var(&id, "id", 0, "Decision id to roll back (required)")
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite", "Path to the SQLite store")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would happen without mutating")
	return c
}

type rollbackOpts struct {
	ID     int64
	DBPath string
	DryRun bool
	Stdout io.Writer
	// Now is injected by tests; defaults to time.Now().UTC().
	Now func() time.Time
}

// runRollback performs the inverse of apply-decision. Failure modes (any of
// which surface as *UsageError so the operator can fix without a crash log):
//
//   - decision id not present in audit.jsonl
//   - latest entry has action='rolled-back' (no-op: already reverted)
//   - deprecated file no longer exists on disk (manual cleanup needed)
//   - canonical path is already occupied (another file was put there since)
func runRollback(ctx context.Context, o rollbackOpts) error {
	if o.Now == nil {
		o.Now = func() time.Time { return time.Now().UTC() }
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.ID <= 0 {
		return learnerr.Usagef("rollback: --id must be > 0")
	}
	if o.DBPath == "" {
		return learnerr.Usagef("rollback: --db-path is required")
	}

	learningDir := audit.LearningDirFromDB(o.DBPath)
	entry, findErr := audit.FindByDecisionID(learningDir, o.ID)
	if findErr != nil {
		return findErr
	}
	if entry == nil {
		return learnerr.Usagef("rollback: no audit entry for decision %d", o.ID)
	}
	if entry.Action == "rolled-back" {
		return learnerr.Usagef("rollback: decision %d is already rolled back (audit entry action=rolled-back)", o.ID)
	}
	if entry.DeprecatedPath == "" || entry.SourcePath == "" {
		return learnerr.Usagef("rollback: decision %d has incomplete audit entry (deprecated_path=%q source_path=%q)",
			o.ID, entry.DeprecatedPath, entry.SourcePath)
	}

	if _, statErr := os.Stat(entry.DeprecatedPath); errors.Is(statErr, os.ErrNotExist) {
		return learnerr.Usagef("rollback: deprecated file %q no longer exists on disk", entry.DeprecatedPath)
	} else if statErr != nil {
		return learnerr.Runtimef("rollback: stat deprecated file %q: %w", entry.DeprecatedPath, statErr)
	}
	if _, statErr := os.Stat(entry.SourcePath); statErr == nil {
		return learnerr.Usagef("rollback: canonical path %q is occupied (another file landed there since apply); resolve manually first", entry.SourcePath)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return learnerr.Runtimef("rollback: stat canonical path %q: %w", entry.SourcePath, statErr)
	}

	if o.DryRun {
		_, _ = fmt.Fprintf(o.Stdout, "Would restore %q → %q (strip deprecation header)\n", entry.DeprecatedPath, entry.SourcePath)
		return nil
	}

	// Read deprecated content + strip header. The header is the first line(s)
	// up to and including the first blank line, conventionally produced by
	// moveToDeprecated as:
	//     > Deprecated YYYY-MM-DD by /learn-refine: merged into <target>
	//     <blank line>
	//     <original content>
	raw, readErr := os.ReadFile(entry.DeprecatedPath) //nolint:gosec // audit-supplied path inside the learning dir
	if readErr != nil {
		return learnerr.Runtimef("rollback: read deprecated %q: %w", entry.DeprecatedPath, readErr)
	}
	restored := stripDeprecationHeader(raw)

	if mkErr := os.MkdirAll(filepath.Dir(entry.SourcePath), 0o750); mkErr != nil {
		return learnerr.Runtimef("rollback: mkdir parent of %q: %w", entry.SourcePath, mkErr)
	}
	if writeErr := os.WriteFile(entry.SourcePath, restored, 0o600); writeErr != nil { //nolint:gosec // audit-supplied canonical path
		return learnerr.Runtimef("rollback: write canonical %q: %w", entry.SourcePath, writeErr)
	}
	if rmErr := os.Remove(entry.DeprecatedPath); rmErr != nil {
		return learnerr.Runtimef("rollback: remove deprecated %q (canonical already restored): %w", entry.DeprecatedPath, rmErr)
	}

	// Insert a `rolled-back` decisions row that references the original
	// decision via candidate_signature so operators can trace the pair.
	st, openErr := store.Open(o.DBPath)
	if openErr != nil {
		return learnerr.Runtimef("rollback: open store: %w", openErr)
	}
	defer func() { _ = st.Close() }()

	now := o.Now().UTC()
	const insertStmt = `
INSERT INTO decisions (candidate_signature, action, target_path, rationale, diff, decided_at)
VALUES (?, 'rolled-back', ?, ?, NULL, ?)`
	rollbackSig := fmt.Sprintf("rollback:decision=%d", o.ID)
	rollbackRat := fmt.Sprintf("Restored %q from %q via /learn-rollback", entry.SourcePath, entry.DeprecatedPath)
	if _, execErr := st.DB().ExecContext(ctx, insertStmt,
		rollbackSig,
		nullIfEmpty(entry.SourcePath),
		rollbackRat,
		now.Format(time.RFC3339Nano),
	); execErr != nil {
		return learnerr.Runtimef("rollback: insert rolled-back decision: %w", execErr)
	}

	// Symmetric audit entry.
	if auditErr := audit.Append(learningDir, audit.Entry{
		Timestamp:          now.Format(time.RFC3339Nano),
		DecisionID:         o.ID, // intentionally repeats the original id
		Action:             "rolled-back",
		SourcePath:         entry.SourcePath,
		DeprecatedPath:     entry.DeprecatedPath,
		MergedInto:         entry.MergedInto,
		CandidateSignature: rollbackSig,
		Rationale:          rollbackRat,
	}); auditErr != nil {
		_, _ = fmt.Fprintf(o.Stdout, "warning: audit log append failed: %v\n", auditErr)
	}

	_, _ = fmt.Fprintf(o.Stdout, "rolled back decision %d: %s → %s\n", o.ID, entry.DeprecatedPath, entry.SourcePath)
	return nil
}

// stripDeprecationHeader removes the leading `> Deprecated ...` line and the
// blank line following it. If the input does not begin with the expected
// header (e.g., an operator edited the file manually before rollback), the
// content is returned unchanged.
func stripDeprecationHeader(raw []byte) []byte {
	text := string(raw)
	if !strings.HasPrefix(text, "> Deprecated ") {
		return raw
	}
	// Drop the first line.
	idx := strings.IndexByte(text, '\n')
	if idx < 0 {
		return raw
	}
	rest := text[idx+1:]
	// Drop one optional blank line.
	rest = strings.TrimPrefix(rest, "\n")
	return []byte(rest)
}
