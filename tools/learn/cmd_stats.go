// stats subcommand implementation (REQ-23). Prints a fixed-schema JSON
// document on stdout summarizing the contents of the learning store. Logs
// (errors, diagnostics) go to stderr via the logging package.
package learn

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newStatsCmd())
	})
}

// useRow is a single row in the top/bottom skill-usage rankings emitted by
// stats. Path is the on-disk SKILL.md path; Uses is the cumulative usage_count
// recorded in skill_usage.
type useRow struct {
	Path string `json:"path"`
	Uses int    `json:"uses"`
}

// statsOutput is the fixed wire schema produced on stdout by `learn stats`.
// Keys mirror the spec verbatim — clients (CI checks, dashboards) parse them
// directly, so renames are breaking changes.
type statsOutput struct {
	Counts struct {
		Events     int `json:"events"`
		Patterns   int `json:"patterns"`
		Candidates int `json:"candidates"`
		Decisions  int `json:"decisions"`
		Skills     int `json:"skills"`
		Memory     int `json:"memory"`
	} `json:"counts"`
	LastExtractAt     string   `json:"last_extract_at"`
	LastNudgeAt       string   `json:"last_nudge_at"`
	NudgeCounter      int      `json:"nudge_counter"`
	TopSkillsByUse    []useRow `json:"top_skills_by_use"`
	BottomSkillsByUse []useRow `json:"bottom_skills_by_use"`
	DBSizeBytes       int64    `json:"db_size_bytes"`
}

// statsOpts is the testable input to runStats. Stdout receives the JSON
// payload; Stderr receives structured log lines.
type statsOpts struct {
	DBPath string
	Stdout io.Writer
	Stderr io.Writer
}

// newStatsCmd builds the `learn stats` cobra command.
func newStatsCmd() *cobra.Command {
	var dbPath string
	var format string

	c := &cobra.Command{
		Use:   "stats",
		Short: "Print learning-store statistics as JSON",
		Long: `stats prints a JSON document on stdout summarizing the contents of the
learning store: row counts per table, timestamps of the last extract and
nudge, the top and bottom skills by usage, and the on-disk DB size.

The JSON schema is fixed (see .specs/learning-loop-harness.md REQ-23) so
downstream consumers (CI checks, dashboards) can rely on it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "" && format != "json" {
				return usagef("stats: --format=%q not supported (only json)", format)
			}
			return runStats(cmd.Context(), statsOpts{
				DBPath: dbPath,
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
		},
	}
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite",
		"Path to the learning SQLite store")
	c.Flags().StringVar(&format, "format", "json",
		"Output format (only 'json' supported)")
	return c
}

// runStats executes the stats subcommand: opens the store, executes the
// fixed-schema queries, marshals JSON to o.Stdout, and emits a single
// structured log line to o.Stderr.
//
// Errors are classified per REQ-22: any failure opening the DB or executing a
// query is a *runtimeError (exit 2). The function never panics on
// nil writers — it falls back to os.Stdout/os.Stderr.
func runStats(ctx context.Context, o statsOpts) error {
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	logger := newLogger(o.Stderr, "stats")

	if o.DBPath == "" {
		return usagef("stats: --db-path must be non-empty")
	}

	// Reject obviously-bad paths before touching SQLite, which would otherwise
	// happily create a new empty DB on the parent dir, masking the failure.
	if _, statErr := os.Stat(filepath.Dir(o.DBPath)); statErr != nil {
		logger.Error("db parent dir not accessible", "db_path", o.DBPath, "err", statErr.Error())
		return runtimef("stats: stat %q: %w", filepath.Dir(o.DBPath), statErr)
	}

	st, openErr := openStore(o.DBPath)
	if openErr != nil {
		// openStore already returns *RuntimeError; preserve it via %w.
		logger.Error("open store failed", "db_path", o.DBPath, "err", openErr.Error())
		return openErr
	}
	defer func() { _ = st.Close() }()

	out, collectErr := collectStats(ctx, st.DB(), o.DBPath)
	if collectErr != nil {
		logger.Error("collect stats failed", "err", collectErr.Error())
		return collectErr
	}

	body, marshalErr := json.MarshalIndent(out, "", "  ")
	if marshalErr != nil {
		logger.Error("marshal stats failed", "err", marshalErr.Error())
		return runtimef("stats: marshal output: %w", marshalErr)
	}
	body = append(body, '\n')
	if _, writeErr := o.Stdout.Write(body); writeErr != nil {
		logger.Error("write stdout failed", "err", writeErr.Error())
		return runtimef("stats: write stdout: %w", writeErr)
	}

	logger.Info("stats emitted",
		"events", out.Counts.Events,
		"patterns", out.Counts.Patterns,
		"candidates", out.Counts.Candidates,
		"decisions", out.Counts.Decisions,
		"skills", out.Counts.Skills,
		"memory", out.Counts.Memory,
		"db_size_bytes", out.DBSizeBytes,
	)
	return nil
}

// collectStats runs the read-only queries that fill the statsOutput struct.
// The function is a pure read path — it never mutates the DB.
func collectStats(ctx context.Context, db *sql.DB, dbPath string) (statsOutput, error) {
	var out statsOutput
	// Match the order of the spec — keeps reviewers honest.
	tables := []struct {
		name string
		dst  *int
	}{
		{"events", &out.Counts.Events},
		{"patterns", &out.Counts.Patterns},
		{"candidates", &out.Counts.Candidates},
		{"decisions", &out.Counts.Decisions},
		{"skill_index", &out.Counts.Skills},
		{"memory_index", &out.Counts.Memory},
	}
	for _, t := range tables {
		if countErr := db.QueryRowContext(ctx,
			`SELECT count(*) FROM `+t.name).Scan(t.dst); countErr != nil {
			return out, runtimef("stats: count %s: %w", t.name, countErr)
		}
	}

	// last_extract_at: max(created_at) over candidates; NULL when empty.
	var lastExtract sql.NullString
	if scanErr := db.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM candidates`).Scan(&lastExtract); scanErr != nil {
		return out, runtimef("stats: max(candidates.created_at): %w", scanErr)
	}
	if lastExtract.Valid {
		out.LastExtractAt = lastExtract.String
	}

	// nudge_state row is seeded at init time with id=1, counter=0, last_nudge_at=NULL.
	var lastNudge sql.NullString
	if scanErr := db.QueryRowContext(ctx,
		`SELECT counter, last_nudge_at FROM nudge_state WHERE id=1`).Scan(&out.NudgeCounter, &lastNudge); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			// Tolerate absence — same shape as a freshly-applied schema where
			// the seed insert ran but a future maintenance op deleted the row.
			out.NudgeCounter = 0
		} else {
			return out, runtimef("stats: read nudge_state: %w", scanErr)
		}
	}
	if lastNudge.Valid {
		out.LastNudgeAt = lastNudge.String
	}

	// Top/bottom skills by use. The ORDER BY ties path ASC for determinism so
	// snapshot tests don't flake when usage_count collides.
	top, topErr := scanUseRows(ctx, db,
		`SELECT path, usage_count FROM skill_usage ORDER BY usage_count DESC, path ASC LIMIT 5`)
	if topErr != nil {
		return out, runtimef("stats: top skills: %w", topErr)
	}
	out.TopSkillsByUse = top

	bottom, bottomErr := scanUseRows(ctx, db,
		`SELECT path, usage_count FROM skill_usage ORDER BY usage_count ASC, path ASC LIMIT 5`)
	if bottomErr != nil {
		return out, runtimef("stats: bottom skills: %w", bottomErr)
	}
	out.BottomSkillsByUse = bottom

	// db_size_bytes is the on-disk file size of the main DB file. WAL/SHM
	// sidecar sizes are intentionally excluded — they're transient.
	if info, statErr := os.Stat(dbPath); statErr == nil {
		out.DBSizeBytes = info.Size()
	} else {
		return out, runtimef("stats: stat db file: %w", statErr)
	}
	return out, nil
}

// scanUseRows iterates a "SELECT path, usage_count ..." query and returns
// the rows as []useRow. Returns an empty slice (never nil) when no rows match,
// so the JSON output is consistently "[]" rather than "null".
func scanUseRows(ctx context.Context, db *sql.DB, query string) ([]useRow, error) {
	rows, queryErr := db.QueryContext(ctx, query)
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { _ = rows.Close() }()

	out := []useRow{}
	for rows.Next() {
		var r useRow
		if scanErr := rows.Scan(&r.Path, &r.Uses); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, r)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return out, nil
}
