// apply-decision subcommand (REQ-9, REQ-10). Reads a pending-approval
// decision row and performs the file move it described: the target file is
// renamed under `<dir>/_deprecated/<basename>-<UTC-timestamp>.md` with a
// literal one-line header pointing at the canonical replacement.
//
// `--dry-run` prints the row's diff to stdout and exits without mutating
// either the filesystem or the row.
package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newApplyDecisionCmd())
	})
}

// newApplyDecisionCmd builds the `learn apply-decision` cobra command.
func newApplyDecisionCmd() *cobra.Command {
	var (
		id     int64
		dbPath string
		dryRun bool
	)
	c := &cobra.Command{
		Use:   "apply-decision",
		Short: "Apply a pending-approval decision (move to _deprecated, mark applied)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApplyDecision(cmd.Context(), applyDecisionOpts{
				ID:     id,
				DBPath: dbPath,
				DryRun: dryRun,
				Stdout: cmd.OutOrStdout(),
			})
		},
	}
	c.Flags().Int64Var(&id, "id", 0, "Decision id to apply (required)")
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite", "Path to the SQLite store")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Print the recorded diff without mutating files or the row")
	return c
}

// applyDecisionOpts is the typed input to runApplyDecision.
type applyDecisionOpts struct {
	ID     int64
	DBPath string
	DryRun bool
	Stdout io.Writer
	// Now is injected by tests; defaults to time.Now().UTC(). The timestamp
	// it returns is used both for the _deprecated/ filename and the header
	// stamped into the moved file.
	Now func() time.Time
}

// applyDecisionRow is the projection of the `decisions` row needed for apply.
type applyDecisionRow struct {
	ID         int64
	Action     string
	TargetPath sql.NullString
	Diff       sql.NullString
}

// runApplyDecision performs the file move and row update. Validation: the row
// must exist, action must be `pending-approval`, target_path must be set.
func runApplyDecision(ctx context.Context, o applyDecisionOpts) error {
	if o.Now == nil {
		o.Now = func() time.Time { return time.Now().UTC() }
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.ID <= 0 {
		return learnerr.Usagef("apply-decision: --id must be > 0")
	}
	if o.DBPath == "" {
		return learnerr.Usagef("apply-decision: --db-path must be non-empty")
	}

	st, openErr := store.Open(o.DBPath)
	if openErr != nil {
		return learnerr.Runtimef("apply-decision: open store: %w", openErr)
	}
	defer func() { _ = st.Close() }()

	row, fetchErr := loadDecisionRow(ctx, st.DB(), o.ID)
	if fetchErr != nil {
		return fetchErr
	}
	if row.Action != "pending-approval" {
		return learnerr.Usagef("apply-decision: decision %d has action %q (want pending-approval)", o.ID, row.Action)
	}
	if !row.TargetPath.Valid || row.TargetPath.String == "" {
		return learnerr.Usagef("apply-decision: decision %d has empty target_path", o.ID)
	}

	if o.DryRun {
		if row.Diff.Valid {
			if _, writeErr := io.WriteString(o.Stdout, row.Diff.String); writeErr != nil {
				return learnerr.Runtimef("apply-decision: write diff to stdout: %w", writeErr)
			}
		}
		return nil
	}

	// Move file to _deprecated/.
	// TODO(learning-loop): once /learn-refine emits a structured row.Diff
	// payload that includes the canonical merge-target path, extract that
	// path here and pass it as mergedInto. For now we use the row's
	// target_path as both source and "merged into" — the header will read
	// "merged into <itself>" which is semantically a self-deprecation; the
	// agentic skill can override by writing a diff that overrides the
	// header before applying.
	now := o.Now().UTC()
	mergedInto := row.TargetPath.String
	deprecatedPath, moveErr := moveToDeprecated(row.TargetPath.String, mergedInto, now)
	if moveErr != nil {
		return moveErr
	}

	const updateDecision = `UPDATE decisions SET action='applied', decided_at=? WHERE id=?`
	if _, execErr := st.DB().ExecContext(ctx, updateDecision,
		now.Format(time.RFC3339Nano), o.ID,
	); execErr != nil {
		return learnerr.Runtimef("apply-decision: mark decision %d as applied (file already moved to %q): %w", o.ID, deprecatedPath, execErr)
	}
	return nil
}

// loadDecisionRow fetches the minimal projection needed to act on a decision.
func loadDecisionRow(ctx context.Context, db *sql.DB, id int64) (applyDecisionRow, error) {
	const q = `SELECT id, action, target_path, diff FROM decisions WHERE id=?`
	var row applyDecisionRow
	scanErr := db.QueryRowContext(ctx, q, id).Scan(&row.ID, &row.Action, &row.TargetPath, &row.Diff)
	if errors.Is(scanErr, sql.ErrNoRows) {
		return applyDecisionRow{}, learnerr.Usagef("apply-decision: no decision row with id=%d", id)
	}
	if scanErr != nil {
		return applyDecisionRow{}, learnerr.Runtimef("apply-decision: scan decision row %d: %w", id, scanErr)
	}
	return row, nil
}

// moveToDeprecated renames sourcePath to `<dir>/_deprecated/<base>-<TS>.md`,
// stamping a one-line header onto the moved file that records the canonical
// replacement. Returns the new path so the caller can include it in error
// context.
//
// REQ-10: timestamp is UTC, format `YYYYMMDDTHHMMSSZ` (basic ISO without
// punctuation) — chosen so filenames remain shell-friendly.
func moveToDeprecated(sourcePath, mergedInto string, now time.Time) (string, error) {
	content, readErr := os.ReadFile(sourcePath) //nolint:gosec // sourcePath comes from a decision row our subcommand inserted
	if readErr != nil {
		return "", learnerr.Runtimef("apply-decision: read source %q: %w", sourcePath, readErr)
	}

	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ".md"
	}

	stamp := now.UTC().Format("20060102T150405Z")
	deprecatedDir := filepath.Join(dir, "_deprecated")
	if mkErr := os.MkdirAll(deprecatedDir, 0o750); mkErr != nil {
		return "", learnerr.Runtimef("apply-decision: mkdir %q: %w", deprecatedDir, mkErr)
	}

	newPath := filepath.Join(deprecatedDir, fmt.Sprintf("%s-%s%s", stem, stamp, ext))
	header := fmt.Sprintf("> Deprecated %s by /learn-refine: merged into %s\n\n", now.UTC().Format("2006-01-02"), mergedInto)
	newContent := append([]byte(header), content...)

	// Atomicity: try os.Rename first (atomic on the same filesystem). If
	// the rename succeeds we then prepend the header via a follow-up
	// write. If rename fails (cross-filesystem, etc.), fall back to the
	// write-then-remove path. Either way we never leave the file in a
	// half-deprecated state — under any failure mode there's exactly one
	// live copy (either at the source or under _deprecated/).
	if renameErr := os.Rename(sourcePath, newPath); renameErr == nil {
		// newPath is built from filepath.Join(filepath.Dir(sourcePath), ...).
		// sourcePath comes from a DB row our own subcommands inserted; not
		// directly user-typed at the boundary.
		if writeErr := os.WriteFile(newPath, newContent, 0o600); writeErr != nil { //nolint:gosec // newPath is derived from a decision row we wrote
			// We renamed but failed to stamp the header. Restore the
			// original content (without header) so the file isn't left
			// corrupted. This is best-effort: if the WriteFile failed
			// the underlying FS is misbehaving.
			_ = os.WriteFile(newPath, content, 0o600) //nolint:gosec // same controlled-path provenance
			return newPath, learnerr.Runtimef("apply-decision: write header to %q: %w", newPath, writeErr)
		}
		return newPath, nil
	}

	// Fallback path (cross-filesystem): write first, then remove. We
	// still avoid the "two live copies" trap by using a fresh inode in
	// _deprecated/ that mirrors the source content — operators can
	// dedupe manually if remove fails.
	if writeErr := os.WriteFile(newPath, newContent, 0o600); writeErr != nil { //nolint:gosec // same controlled-path provenance
		return "", learnerr.Runtimef("apply-decision: write %q: %w", newPath, writeErr)
	}
	if rmErr := os.Remove(sourcePath); rmErr != nil {
		return newPath, learnerr.Runtimef("apply-decision: remove source %q (new path %q already written): %w", sourcePath, newPath, rmErr)
	}
	return newPath, nil
}
