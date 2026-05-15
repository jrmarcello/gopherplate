// complete-task subcommand implementation (REQ-1, REQ-11).
//
// Records the completion of a spec via the SDD harness: appends an `events`
// row and atomically bumps the nudge counter. When the counter reaches or
// exceeds the configured threshold, prints `LEARN_NUDGE_DUE=true` on stderr
// so the stop-learn.sh hook can surface the cue to the user. Counter reset
// is the responsibility of the `nudge` subcommand (not this one).
package learn

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newCompleteTaskCmd())
	})
}

// completeOpts is the input to runCompleteTask. Fields with empty zero values
// fall back to canonical defaults: DBPath -> "<dir>/db.sqlite",
// ConfigPath -> "<dir>/config.yml", Stderr -> os.Stderr, Now -> time.Now.
//
// Config, when non-nil, short-circuits loadConfig — useful for tests that
// need to control NudgeThreshold deterministically without writing a yaml file.
type completeOpts struct {
	LearningDir string
	DBPath      string
	ConfigPath  string
	SpecPath    string
	SessionID   string
	CommitSHA   string
	TasksCount  int

	// Injected for testability.
	Config *loopConfig
	Stderr io.Writer
	Now    func() time.Time
}

// newCompleteTaskCmd builds the `learn complete-task` cobra command.
func newCompleteTaskCmd() *cobra.Command {
	var (
		learningDir string
		dbPath      string
		configPath  string
		specPath    string
		sessionID   string
		commitSHA   string
		tasksCount  int
	)

	c := &cobra.Command{
		Use:   "complete-task",
		Short: "Record a spec completion and bump the nudge counter",
		Long: `complete-task appends a row to the events table for the given spec/session
and atomically increments the nudge counter. When the counter reaches the
configured threshold, prints LEARN_NUDGE_DUE=true on stderr so the
stop-learn.sh hook can surface the cue. Reset of the counter is performed
by the nudge subcommand, not here.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if specPath == "" {
				return usagef("complete-task: --spec is required")
			}
			if sessionID == "" {
				return usagef("complete-task: --session is required")
			}
			return runCompleteTask(cmd.Context(), completeOpts{
				LearningDir: learningDir,
				DBPath:      dbPath,
				ConfigPath:  configPath,
				SpecPath:    specPath,
				SessionID:   sessionID,
				CommitSHA:   commitSHA,
				TasksCount:  tasksCount,
				Stderr:      cmd.ErrOrStderr(),
				Now:         time.Now,
			})
		},
	}
	c.Flags().StringVar(&learningDir, "dir", ".claude/learning",
		"Learning directory (default .claude/learning)")
	c.Flags().StringVar(&dbPath, "db-path", "",
		"Override the DB path (default <dir>/db.sqlite)")
	c.Flags().StringVar(&configPath, "config", "",
		"Override the config path (default <dir>/config.yml)")
	c.Flags().StringVar(&specPath, "spec", "",
		"Path to the spec file that just completed (required)")
	c.Flags().StringVar(&sessionID, "session", "",
		"Session identifier emitted by the harness (required)")
	c.Flags().StringVar(&commitSHA, "commit-sha", "",
		"Commit SHA associated with the completion (optional)")
	c.Flags().IntVar(&tasksCount, "tasks-count", 0,
		"Number of tasks completed in the session (optional)")

	// Pre-flight required-flag validation is done in RunE so the missing-flag
	// error becomes a *usageError (exit 1) instead of cobra's default
	// error type. We still call MarkFlagRequired so `--help` displays the
	// "(required)" hint; cobra will short-circuit before RunE in that case,
	// but the RunE check above keeps the contract explicit and testable.
	_ = c.MarkFlagRequired("spec")
	_ = c.MarkFlagRequired("session")
	return c
}

// runCompleteTask performs the side-effects of `learn complete-task`:
//
//  1. Load config (path -> ConfigPath, else <LearningDir>/config.yml) unless
//     the caller already supplied a *Config in opts.
//  2. Open the store (DBPath, else <LearningDir>/db.sqlite).
//  3. Insert an `events` row with the supplied identifiers and a UTC RFC3339
//     completion timestamp.
//  4. Atomically increment the nudge counter (RETURNING on UPDATE).
//  5. When the new counter is >= NudgeThreshold, emit LEARN_NUDGE_DUE=true to
//     the injected Stderr. The counter itself is NOT reset here.
//
// All IO failures are wrapped as *runtimeError so the dispatcher
// exits with code 2.
func runCompleteTask(ctx context.Context, o completeOpts) error {
	if o.SpecPath == "" {
		return usagef("complete-task: spec path must be non-empty")
	}
	if o.SessionID == "" {
		return usagef("complete-task: session id must be non-empty")
	}

	stderr := o.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	now := o.Now
	if now == nil {
		now = time.Now
	}

	cfg := o.Config
	if cfg == nil {
		configPath := o.ConfigPath
		if configPath == "" && o.LearningDir != "" {
			configPath = filepath.Join(o.LearningDir, "config.yml")
		}
		loaded, loadErr := loadConfig(configPath)
		if loadErr != nil {
			return runtimef("complete-task: load config: %w", loadErr)
		}
		cfg = loaded
	}

	dbPath := o.DBPath
	if dbPath == "" {
		if o.LearningDir == "" {
			return usagef("complete-task: --db-path or --dir must be set")
		}
		dbPath = filepath.Join(o.LearningDir, "db.sqlite")
	}

	st, openErr := openStore(dbPath)
	if openErr != nil {
		return runtimef("complete-task: open store: %w", openErr)
	}
	defer func() { _ = st.Close() }()

	ev := storeEvent{
		SpecPath:    o.SpecPath,
		SessionID:   o.SessionID,
		CompletedAt: now().UTC().Format(time.RFC3339),
		CommitSHA:   o.CommitSHA,
		TasksCount:  o.TasksCount,
		Outcome:     "success",
	}
	if insertErr := st.InsertEvent(ctx, ev); insertErr != nil {
		return runtimef("complete-task: insert event: %w", insertErr)
	}

	newCounter, incrementErr := st.IncrementNudgeCounter(ctx)
	if incrementErr != nil {
		return runtimef("complete-task: increment nudge counter: %w", incrementErr)
	}

	if newCounter >= cfg.NudgeThreshold {
		if _, writeErr := fmt.Fprintf(stderr, "LEARN_NUDGE_DUE=true counter=%d\n", newCounter); writeErr != nil {
			return runtimef("complete-task: emit nudge signal: %w", writeErr)
		}
	}
	return nil
}
