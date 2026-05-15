// nudge-tick subcommand implementation (REQ-11, REQ-12).
//
// Two modes:
//
//  1. Without --reset: query skill_index/memory_index joined with
//     skill_usage/memory_usage for rows whose indexed_at <= now-2*TTL AND
//     (last_used_at IS NULL OR last_used_at <= now-TTL). Emit deprecation
//     candidates as a JSON document on stdout.
//  2. With --reset: zero nudge_state.counter and stamp last_nudge_at = now
//     in a single atomic UPDATE. Emit "{}\n" on stdout. Used by the
//     /learn-nudge skill after presenting deprecation candidates.
//
// Time is injected via opts.Now so tests are deterministic; the cobra
// command defaults to time.Now.UTC.
package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newNudgeTickCmd())
	})
}

// nudgeTickOpts is the testable input to runNudgeTick. Tests inject Config,
// Now, and writers directly; the cobra command builds it from flags.
type nudgeTickOpts struct {
	DBPath     string
	ConfigPath string
	Reset      bool
	TTLDays    int // 0 = use config
	Format     string
	Config     *config.Config
	Stdout     io.Writer
	Stderr     io.Writer
	Now        func() time.Time
}

// newNudgeTickCmd builds the `learn nudge-tick` cobra command.
func newNudgeTickCmd() *cobra.Command {
	var (
		dbPath     string
		configPath string
		reset      bool
		ttlDays    int
		format     string
	)

	c := &cobra.Command{
		Use:   "nudge-tick",
		Short: "Emit deprecation candidates or reset the nudge counter",
		Long: `nudge-tick has two modes.

Without --reset (default): scan skill_index/memory_index joined with their
usage tables and emit rows whose indexed_at <= now - 2*ttl_days AND
last_used_at <= now - ttl_days (NULL counts as "never used") as a JSON
document. Used by the /learn-nudge skill to surface deprecation candidates.

With --reset: zero nudge_state.counter AND set last_nudge_at = now in a
single atomic UPDATE, then print "{}\n". The skill calls this after
presenting candidates so the next nudge fires only after another
NudgeThreshold loops.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNudgeTick(cmd.Context(), nudgeTickOpts{
				DBPath:     dbPath,
				ConfigPath: configPath,
				Reset:      reset,
				TTLDays:    ttlDays,
				Format:     format,
				Stdout:     cmd.OutOrStdout(),
				Stderr:     cmd.ErrOrStderr(),
				Now:        func() time.Time { return time.Now().UTC() },
			})
		},
	}
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite", "Path to the learning SQLite store")
	c.Flags().StringVar(&configPath, "config", ".claude/learning/config.yml", "Path to learning config")
	c.Flags().BoolVar(&reset, "reset", false, "Reset nudge counter to 0 and stamp last_nudge_at")
	c.Flags().IntVar(&ttlDays, "ttl-days", 0, "Override config.ttl_days (0 = use config)")
	c.Flags().StringVar(&format, "format", "json", "Output format (only 'json' is supported)")
	return c
}

// nudgeTickCandidate mirrors store.DeprecationCandidate but uses JSON tags
// matching the documented contract (snake_case, RFC3339, null last_used_at).
type nudgeTickCandidate struct {
	Kind       string  `json:"kind"`
	Path       string  `json:"path"`
	IndexedAt  string  `json:"indexed_at"`
	LastUsedAt *string `json:"last_used_at"`
	UsageCount int     `json:"usage_count"`
}

// nudgeTickReport is the JSON document emitted on stdout in default mode.
type nudgeTickReport struct {
	CheckedAt  string               `json:"checked_at"`
	TTLDays    int                  `json:"ttl_days"`
	Candidates []nudgeTickCandidate `json:"candidates"`
}

// runNudgeTick is the test-friendly entry point. Validates flags, loads
// config when not injected, then dispatches to reset or list mode.
func runNudgeTick(ctx context.Context, o nudgeTickOpts) error {
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.Now == nil {
		o.Now = func() time.Time { return time.Now().UTC() }
	}
	if o.DBPath == "" {
		return learnerr.Usagef("nudge-tick: --db-path must be non-empty")
	}
	if o.Format == "" {
		o.Format = "json"
	}
	if o.Format != "json" {
		return learnerr.Usagef("nudge-tick: unsupported --format %q (only 'json')", o.Format)
	}

	cfg := o.Config
	if cfg == nil {
		loaded, loadErr := config.Load(o.ConfigPath)
		if loadErr != nil {
			return learnerr.Runtimef("nudge-tick: load config: %w", loadErr)
		}
		cfg = loaded
	}

	ttl := cfg.TTLDays
	if o.TTLDays > 0 {
		ttl = o.TTLDays
	}
	if ttl < 1 {
		return learnerr.Usagef("nudge-tick: ttl_days must be >= 1, got %d", ttl)
	}

	st, openErr := store.Open(o.DBPath)
	if openErr != nil {
		return learnerr.Runtimef("nudge-tick: open store: %w", openErr)
	}
	defer func() { _ = st.Close() }()

	now := o.Now().UTC().Format(time.RFC3339)

	if o.Reset {
		if markErr := st.MarkNudgeRun(ctx, now); markErr != nil {
			return learnerr.Runtimef("nudge-tick: mark nudge run: %w", markErr)
		}
		// Empty JSON object signals "reset complete, no payload".
		if _, writeErr := io.WriteString(o.Stdout, "{}\n"); writeErr != nil {
			return learnerr.Runtimef("nudge-tick: write stdout: %w", writeErr)
		}
		return nil
	}

	rows, listErr := st.ListDeprecationCandidates(ctx, now, ttl)
	if listErr != nil {
		return learnerr.Runtimef("nudge-tick: list candidates: %w", listErr)
	}

	report := nudgeTickReport{
		CheckedAt:  now,
		TTLDays:    ttl,
		Candidates: make([]nudgeTickCandidate, 0, len(rows)),
	}
	for _, r := range rows {
		report.Candidates = append(report.Candidates, nudgeTickCandidate{
			Kind:       r.Kind,
			Path:       r.Path,
			IndexedAt:  r.IndexedAt,
			LastUsedAt: r.LastUsedAt,
			UsageCount: r.UsageCount,
		})
	}

	enc := json.NewEncoder(o.Stdout)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(report); encErr != nil {
		return learnerr.Runtimef("nudge-tick: encode json: %w", encErr)
	}
	return nil
}
