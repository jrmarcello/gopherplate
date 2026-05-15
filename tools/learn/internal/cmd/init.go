// init subcommand implementation. Creates the SQLite store, default config,
// and local .gitignore inside the learning directory. Idempotent — re-running
// preserves data and user edits.
package cmd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newInitCmd())
	})
}

// newInitCmd builds the `learn init` cobra command.
func newInitCmd() *cobra.Command {
	var dbPath, configPath, learningDir string

	c := &cobra.Command{
		Use:   "init",
		Short: "Initialize the learning loop store and config",
		Long: `init creates the SQLite store, writes a default config.yml, and ensures
the learning directory is ignored by git. Idempotent — running init on an
existing learning directory does not destroy data.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(initOpts{
				LearningDir: learningDir,
				DBPath:      dbPath,
				ConfigPath:  configPath,
			})
		},
	}
	c.Flags().StringVar(&learningDir, "dir", ".claude/learning",
		"Learning directory (default .claude/learning)")
	c.Flags().StringVar(&dbPath, "db-path", "",
		"Override the DB path (default <dir>/db.sqlite)")
	c.Flags().StringVar(&configPath, "config", "",
		"Override the config path (default <dir>/config.yml)")
	return c
}

// initOpts is the input to runInit. Empty DBPath/ConfigPath fields default
// to <LearningDir>/db.sqlite and <LearningDir>/config.yml.
type initOpts struct {
	LearningDir string
	DBPath      string
	ConfigPath  string
}

// runInit performs the side-effects of `learn init`:
//
//  1. Create LearningDir (MkdirAll, 0o755). EACCES surfaces as RuntimeError.
//  2. Write default config.yml if absent (preserve if present).
//  3. Open the SQLite store at DBPath — schema apply is idempotent.
//  4. Write a local .gitignore inside LearningDir if absent.
//
// runInit takes no context because it is bounded by the local filesystem and
// is short-lived; future subcommands that perform I/O against external
// systems will accept the cobra-provided context.
func runInit(o initOpts) error {
	if o.LearningDir == "" {
		return learnerr.Usagef("init: --dir must be non-empty")
	}

	// 0o700: only the owning user reads/writes the learning dir.
	// db.sqlite contains sanitized-but-still-sensitive transcripts/patterns;
	// defense-in-depth against multi-user host exposure.
	if mkdirErr := os.MkdirAll(o.LearningDir, 0o700); mkdirErr != nil {
		return learnerr.Runtimef("init: create %q: %w", o.LearningDir, mkdirErr)
	}

	dbPath := o.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(o.LearningDir, "db.sqlite")
	}
	configPath := o.ConfigPath
	if configPath == "" {
		configPath = filepath.Join(o.LearningDir, "config.yml")
	}

	if writeErr := writeDefaultConfigIfAbsent(configPath); writeErr != nil {
		return writeErr
	}

	st, openErr := store.Open(dbPath)
	if openErr != nil {
		return learnerr.Runtimef("init: open store: %w", openErr)
	}
	if closeErr := st.Close(); closeErr != nil {
		return learnerr.Runtimef("init: close store: %w", closeErr)
	}

	if gitignoreErr := writeLocalGitignoreIfAbsent(o.LearningDir); gitignoreErr != nil {
		return gitignoreErr
	}

	return nil
}

// writeDefaultConfigIfAbsent serializes config.DefaultConfig() to YAML and
// writes it to path only when no file exists there. Existing files are left
// untouched so user edits survive re-running init.
func writeDefaultConfigIfAbsent(path string) error {
	if _, statErr := os.Stat(path); statErr == nil {
		return nil // preserve existing config
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return learnerr.Runtimef("init: stat %q: %w", path, statErr)
	}

	cfg := config.DefaultConfig()
	body, marshalErr := yaml.Marshal(cfg)
	if marshalErr != nil {
		return learnerr.Runtimef("init: marshal default config: %w", marshalErr)
	}
	if len(body) == 0 || body[len(body)-1] != '\n' {
		body = append(body, '\n')
	}
	if writeErr := os.WriteFile(path, body, 0o600); writeErr != nil {
		return learnerr.Runtimef("init: write %q: %w", path, writeErr)
	}
	return nil
}

// writeLocalGitignoreIfAbsent creates a small .gitignore inside the learning
// directory that excludes the SQLite DB and the log file. Repo-root
// .gitignore wiring is handled separately (TASK-38); this file is local so
// the rule travels with the directory itself.
func writeLocalGitignoreIfAbsent(learningDir string) error {
	path := filepath.Join(learningDir, ".gitignore")
	if _, statErr := os.Stat(path); statErr == nil {
		return nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return learnerr.Runtimef("init: stat %q: %w", path, statErr)
	}
	const body = "db.sqlite\nlearn.log\n"
	if writeErr := os.WriteFile(path, []byte(body), 0o600); writeErr != nil {
		return learnerr.Runtimef("init: write %q: %w", path, writeErr)
	}
	return nil
}
