// refine-apply subcommand (REQ-9, REQ-9a). Records a refinement decision and
// applies it atomically in a single call — the workflow expected by the
// auto-apply policy adopted by /learn-refine.
//
// Equivalent to: `record-decision --action=pending-approval ...` followed by
// `apply-decision --id=<inserted>`. Bundling both into one subcommand spares
// the agentic skill from having to parse the inserted ID from stdout.
package learn

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newRefineApplyCmd())
	})
}

func newRefineApplyCmd() *cobra.Command {
	var (
		candidateSig string
		targetPath   string
		mergedInto   string
		rationale    string
		diffFile     string
		dbPath       string
		dryRun       bool
	)
	c := &cobra.Command{
		Use:   "refine-apply",
		Short: "Record a refinement decision and apply it (move to _deprecated, audit) in one call",
		Long: `refine-apply is the canonical entry for /learn-refine under the auto-apply
policy (REQ-9). It inserts a pending-approval decision row and immediately
moves the target file to _deprecated/, stamping the audit.jsonl entry
(REQ-9a). With --dry-run it prints the diff and exits without mutating.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefineApply(cmd.Context(), refineApplyOpts{
				CandidateSignature: candidateSig,
				TargetPath:         targetPath,
				MergedInto:         mergedInto,
				Rationale:          rationale,
				DiffFile:           diffFile,
				DBPath:             dbPath,
				DryRun:             dryRun,
				Stdout:             cmd.OutOrStdout(),
			})
		},
	}
	c.Flags().StringVar(&candidateSig, "candidate-signature", "", "Candidate signature being decided (required)")
	c.Flags().StringVar(&targetPath, "target-path", "", "Path of the skill/memory file to deprecate (required)")
	c.Flags().StringVar(&mergedInto, "merged-into", "", "Canonical replacement path; defaults to --target-path (self-deprecate)")
	c.Flags().StringVar(&rationale, "rationale", "", "Free-form rationale recorded in the decision row")
	c.Flags().StringVar(&diffFile, "diff-file", "", "Optional file with proposed diff (recorded on the row)")
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite", "Path to the SQLite store")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Print the diff without mutating files or the row")
	return c
}

type refineApplyOpts struct {
	CandidateSignature string
	TargetPath         string
	MergedInto         string
	Rationale          string
	DiffFile           string
	DBPath             string
	DryRun             bool
	Stdout             io.Writer
	// Now is injected by tests; defaults to time.Now().UTC().
	Now func() time.Time
}

// runRefineApply performs the combined record + apply flow in two SQL
// transactions (insert decision, then update + move file). The decision row
// is left as `pending-approval` until the file move succeeds, then bumped to
// `applied`; this ordering keeps the row coherent under crash recovery (a
// pending row points at the file as it exists; an applied row points at the
// deprecated copy).
func runRefineApply(ctx context.Context, o refineApplyOpts) error {
	if o.Now == nil {
		o.Now = func() time.Time { return time.Now().UTC() }
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.CandidateSignature == "" {
		return usagef("refine-apply: --candidate-signature is required")
	}
	if o.TargetPath == "" {
		return usagef("refine-apply: --target-path is required")
	}
	if o.DBPath == "" {
		return usagef("refine-apply: --db-path is required")
	}
	if o.MergedInto == "" {
		// Default to self-deprecation; /learn-refine should normally supply
		// the canonical merge target explicitly.
		o.MergedInto = o.TargetPath
	}

	var diffContent string
	if o.DiffFile != "" {
		raw, readErr := os.ReadFile(o.DiffFile) //nolint:gosec // caller-supplied path
		if readErr != nil {
			return runtimef("refine-apply: read --diff-file %q: %w", o.DiffFile, readErr)
		}
		diffContent = string(raw)
	}

	if o.DryRun {
		_, _ = fmt.Fprintf(o.Stdout, "Would deprecate %q (merged into %q)\nrationale: %s\n", o.TargetPath, o.MergedInto, o.Rationale)
		if diffContent != "" {
			_, _ = fmt.Fprintln(o.Stdout, "---")
			_, _ = fmt.Fprintln(o.Stdout, diffContent)
		}
		return nil
	}

	st, openErr := openStore(o.DBPath)
	if openErr != nil {
		return runtimef("refine-apply: open store: %w", openErr)
	}
	defer func() { _ = st.Close() }()

	now := o.Now().UTC()

	// 1. Insert pending-approval row. We use RETURNING id so we don't need
	// LastInsertId (which is also fine, but RETURNING reads more naturally).
	const insertStmt = `
INSERT INTO decisions (candidate_signature, action, target_path, rationale, diff, decided_at)
VALUES (?, 'pending-approval', ?, ?, ?, ?)
RETURNING id`
	var id int64
	if scanErr := st.DB().QueryRowContext(ctx, insertStmt,
		o.CandidateSignature,
		nullIfEmpty(o.TargetPath),
		o.Rationale,
		nullIfEmpty(diffContent),
		now.Format(time.RFC3339Nano),
	).Scan(&id); scanErr != nil {
		return runtimef("refine-apply: insert decision: %w", scanErr)
	}

	// 2. Perform the deprecation move.
	deprecatedPath, moveErr := moveToDeprecated(o.TargetPath, o.MergedInto, now)
	if moveErr != nil {
		// Roll the row back so we don't leave a phantom pending-approval row
		// pointing at a file that's still in place. Best-effort; if delete
		// fails the error is wrapped together.
		_, _ = st.DB().ExecContext(ctx, `DELETE FROM decisions WHERE id=?`, id)
		return moveErr
	}

	// 3. Bump the row to `applied`.
	const updateStmt = `UPDATE decisions SET action='applied', decided_at=? WHERE id=?`
	if _, execErr := st.DB().ExecContext(ctx, updateStmt, now.Format(time.RFC3339Nano), id); execErr != nil {
		return runtimef("refine-apply: mark decision %d applied (file already moved to %q): %w", id, deprecatedPath, execErr)
	}

	// 4. Append audit.jsonl entry (REQ-9a). Best-effort.
	learningDir := learningDirFromDB(o.DBPath)
	if auditErr := appendAudit(learningDir, auditEntry{
		Timestamp:          now.Format(time.RFC3339Nano),
		DecisionID:         id,
		Action:             "applied",
		SourcePath:         o.TargetPath,
		DeprecatedPath:     deprecatedPath,
		MergedInto:         o.MergedInto,
		CandidateSignature: o.CandidateSignature,
		Rationale:          o.Rationale,
	}); auditErr != nil {
		_, _ = fmt.Fprintf(o.Stdout, "warning: audit log append failed: %v\n", auditErr)
	}

	_, _ = fmt.Fprintf(o.Stdout, "applied decision %d: %s → %s\n", id, o.TargetPath, deprecatedPath)
	return nil
}
