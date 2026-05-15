// track-use subcommand implementation (REQ-17).
//
// Records that one or more skill/memory artifacts were just consumed by the
// agent: increments usage_count and bumps last_used_at on skill_usage or
// memory_usage, depending on which index the path belongs to. Paths that
// don't match any indexed artifact are logged at WARN level and skipped —
// the command always exits 0 (best-effort), so the UserPromptSubmit hook
// can call it without risking a non-zero exit blocking the prompt.
package cmd

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/logging"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newTrackUseCmd())
	})
}

// trackUseOpts is the testable input to runTrackUse. Now allows tests to pin
// the timestamp written into last_used_at.
type trackUseOpts struct {
	DBPath string
	Paths  []string
	Now    func() time.Time
	Stderr io.Writer
}

// newTrackUseCmd builds the `learn track-use` cobra command.
func newTrackUseCmd() *cobra.Command {
	var (
		dbPath   string
		pathsArg string
	)

	c := &cobra.Command{
		Use:   "track-use",
		Short: "Increment usage_count for the given skill/memory paths",
		Long: `track-use atomically increments usage_count and updates last_used_at on
skill_usage or memory_usage for each path passed via --paths. Unknown paths
(not present in skill_index or memory_index) are logged as a warning and
skipped; the command always exits 0 (best-effort).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := splitPaths(pathsArg)
			return runTrackUse(cmd.Context(), trackUseOpts{
				DBPath: dbPath,
				Paths:  paths,
				Now:    time.Now,
				Stderr: cmd.ErrOrStderr(),
			})
		},
	}
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite", "Path to the learning SQLite store")
	c.Flags().StringVar(&pathsArg, "paths", "", "Comma-separated list of skill/memory paths just consumed")
	return c
}

// splitPaths trims and de-empties a comma-separated string. Empty input
// yields an empty slice (not nil) so callers can compare lengths uniformly.
func splitPaths(arg string) []string {
	if arg == "" {
		return []string{}
	}
	parts := strings.Split(arg, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// runTrackUse opens the store and increments usage counters for each path.
// Unknown paths are logged as warnings via slog and skipped — the function
// returns nil regardless (best-effort, REQ-17 / TC-UC-25).
func runTrackUse(ctx context.Context, o trackUseOpts) error {
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.DBPath == "" {
		return learnerr.Usagef("track-use: --db-path must be non-empty")
	}
	if len(o.Paths) == 0 {
		// Empty --paths is a no-op success. The hook may legitimately pass
		// an empty list when no artifacts were referenced.
		return nil
	}

	logger := logging.New(o.Stderr, "track-use")

	st, openErr := store.Open(o.DBPath)
	if openErr != nil {
		return learnerr.Runtimef("track-use: open store: %w", openErr)
	}
	defer func() { _ = st.Close() }()

	now := o.Now().UTC().Format(time.RFC3339)

	for _, path := range o.Paths {
		bumped, bumpErr := st.IncrementSkillUsage(ctx, path, now)
		if bumpErr != nil {
			return learnerr.Runtimef("track-use: increment skill_usage %q: %w", path, bumpErr)
		}
		if bumped {
			continue
		}
		bumpedMem, bumpMemErr := st.IncrementMemoryUsage(ctx, path, now)
		if bumpMemErr != nil {
			return learnerr.Runtimef("track-use: increment memory_usage %q: %w", path, bumpMemErr)
		}
		if bumpedMem {
			continue
		}
		// Path is neither a known skill nor a known memory — log and continue.
		logger.LogAttrs(ctx, slog.LevelWarn, "track-use missing path",
			slog.String("path", path),
		)
	}
	return nil
}
