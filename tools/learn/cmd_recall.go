// recall subcommand implementation (REQ-14, REQ-14a, REQ-15, REQ-16).
//
// Recalls top-K relevant skills/memory/patterns for a given prompt by
// delegating to internal/recallMatches and formatting the result either as
// the canonical system-reminder template (consumed by the UserPromptSubmit
// hook) or as a JSON array (for manual / programmatic use).
//
// Silent-by-design: prompts shorter than cfg.MinPromptLenForRecall and result
// sets with zero matches both produce no output and exit 0 — this is what
// keeps the hook from polluting Claude's context window (REQ-15).
package learn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newRecallCmd())
	})
}

// recallOpts is the testable input to runRecall. Zero values fall back to
// canonical defaults (see runRecall body for the resolution order).
type recallOpts struct {
	Prompt     string
	DBPath     string
	ConfigPath string
	TopK       int
	MaxTokens  int
	MinScore   float64
	KindFilter string
	Since      time.Duration
	Format     string // "system-reminder" | "json"

	Stdout io.Writer
	Stderr io.Writer
	Config *loopConfig

	// Now is injected for deterministic SinceMtime filtering in tests.
	// Defaults to time.Now when nil. Forwarded to recallOptions.
	Now func() time.Time
}

// newRecallCmd builds the `learn recall` cobra command.
func newRecallCmd() *cobra.Command {
	var (
		prompt     string
		dbPath     string
		configPath string
		topK       int
		maxTokens  int
		minScore   float64
		kindFilter string
		since      time.Duration
		format     string
	)

	c := &cobra.Command{
		Use:   "recall",
		Short: "Recall top-K skills/memory/patterns relevant to a prompt",
		Long: `recall queries the learning store's FTS indexes (skill_fts, memory_fts,
pattern_fts) for matches against the given prompt and prints a ranked list of
artifacts. Silent-by-design: short prompts (below min_prompt_len_for_recall)
and empty result sets produce no output and exit 0.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config first so flag defaults can be sourced from it. This
			// preserves the contract that explicit `--top-k=0` etc. surface
			// as usage errors (TC-UC-22c) — defaults are baked into the flag
			// value, never into runRecall's behavior.
			cfg, loadErr := loadConfig(configPath)
			if loadErr != nil {
				return runtimef("recall: load config: %w", loadErr)
			}
			// Apply config defaults to flags that the user did not change.
			if !cmd.Flags().Changed("top-k") {
				topK = cfg.RecallTopK
			}
			if !cmd.Flags().Changed("max-tokens") {
				maxTokens = cfg.RecallMaxTokens
			}
			if !cmd.Flags().Changed("min-score") {
				minScore = cfg.RecallMinScore
			}
			return runRecall(cmd.Context(), recallOpts{
				Prompt:     prompt,
				DBPath:     dbPath,
				ConfigPath: configPath,
				TopK:       topK,
				MaxTokens:  maxTokens,
				MinScore:   minScore,
				KindFilter: kindFilter,
				Since:      since,
				Format:     format,
				Stdout:     cmd.OutOrStdout(),
				Stderr:     cmd.ErrOrStderr(),
				Config:     cfg,
			})
		},
	}
	c.Flags().StringVar(&prompt, "prompt", "", "Prompt text to search against (required)")
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite", "Path to the learning SQLite store")
	c.Flags().StringVar(&configPath, "config", ".claude/learning/config.yml", "Path to learning config (for defaults)")
	c.Flags().IntVar(&topK, "top-k", 0, "Maximum number of matches to return (default from config)")
	c.Flags().IntVar(&maxTokens, "max-tokens", 0, "Cumulative summary character budget (default from config)")
	c.Flags().Float64Var(&minScore, "min-score", 0, "Minimum BM25 score (default from config)")
	c.Flags().StringVar(&kindFilter, "kind", "", "Restrict to one of: skill|memory|pattern (empty = all)")
	c.Flags().DurationVar(&since, "since", 0, "Restrict to artifacts indexed within this duration (0 = no filter)")
	c.Flags().StringVar(&format, "format", "system-reminder", "Output format: system-reminder|json")
	return c
}

// runRecall is the testable entry point for the `learn recall` subcommand.
// It loads config (unless o.Config is already set), applies defaults, calls
// recallMatches, and formats the output per o.Format.
func runRecall(ctx context.Context, o recallOpts) error {
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}

	cfg := o.Config
	if cfg == nil {
		loaded, loadErr := loadConfig(o.ConfigPath)
		if loadErr != nil {
			return runtimef("recall: load config: %w", loadErr)
		}
		cfg = loaded
	}

	// Validate explicit flag values per TC-UC-22c, 22d, 23a, 23b, 23c.
	// runRecall trusts its caller to have already substituted config defaults
	// into TopK/MaxTokens/MinScore; an explicit 0 or negative value is a usage
	// error, never silently corrected.
	if o.TopK < 1 {
		return usagef("recall: --top-k must be >= 1, got %d", o.TopK)
	}
	if o.MaxTokens < 1 {
		return usagef("recall: --max-tokens must be >= 1, got %d", o.MaxTokens)
	}
	if o.MinScore < 0 {
		return usagef("recall: --min-score must be >= 0, got %v", o.MinScore)
	}
	if o.Since < 0 {
		return usagef("recall: --since must be >= 0, got %s", o.Since)
	}
	switch o.KindFilter {
	case "", recallKindSkill, recallKindMemory, recallKindPattern:
		// ok
	default:
		return usagef(
			"recall: --kind=%q invalid (valid: skill, memory, pattern)",
			o.KindFilter,
		)
	}
	switch o.Format {
	case "", "system-reminder", "json":
		// ok
	default:
		return usagef("recall: --format=%q invalid (valid: system-reminder, json)", o.Format)
	}

	// REQ-14 / TC-UC-20: short prompts short-circuit silently.
	if len(o.Prompt) < cfg.MinPromptLenForRecall {
		return nil
	}

	if o.DBPath == "" {
		return usagef("recall: --db-path must be non-empty")
	}
	st, openErr := openStore(o.DBPath)
	if openErr != nil {
		return runtimef("recall: open store: %w", openErr)
	}
	defer func() { _ = st.Close() }()

	matches, recErr := recallMatches(ctx, st.DB(), recallOptions{
		Prompt:     o.Prompt,
		TopK:       o.TopK,
		MaxTokens:  o.MaxTokens,
		MinScore:   o.MinScore,
		KindFilter: o.KindFilter,
		SinceMtime: o.Since,
		Now:        o.Now,
	})
	if recErr != nil {
		return recErr
	}

	// REQ-15 / TC-UC-15: empty results => silent exit.
	if len(matches) == 0 {
		return nil
	}

	format := o.Format
	if format == "" {
		format = "system-reminder"
	}
	switch format {
	case "json":
		return writeJSON(o.Stdout, matches)
	case "system-reminder":
		return writeSystemReminder(o.Stdout, matches)
	}
	return nil
}

// writeJSON marshals matches as a pretty-printed JSON array. The trailing
// newline keeps the output well-behaved when piped into other tools.
func writeJSON(w io.Writer, matches []recallMatch) error {
	body, marshalErr := json.MarshalIndent(matches, "", "  ")
	if marshalErr != nil {
		return runtimef("recall: marshal json: %w", marshalErr)
	}
	body = append(body, '\n')
	if _, writeErr := w.Write(body); writeErr != nil {
		return runtimef("recall: write stdout: %w", writeErr)
	}
	return nil
}

// writeSystemReminder formats matches as the canonical REQ-14a template.
// The exact byte layout is contractual — hook tests parse this output, and the
// template must remain stable across releases.
func writeSystemReminder(w io.Writer, matches []recallMatch) error {
	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	fmt.Fprintf(&b, "Learning loop recall — %d relevant artifacts:\n", len(matches))
	for _, m := range matches {
		fmt.Fprintf(&b, "- %s (%s, score=%.2f): %s\n", m.Path, m.Kind, m.Score, m.Summary)
	}
	b.WriteString("Read the file(s) above if relevant; ignore otherwise.\n")
	b.WriteString("</system-reminder>\n")
	if _, writeErr := w.Write([]byte(b.String())); writeErr != nil {
		return runtimef("recall: write stdout: %w", writeErr)
	}
	return nil
}
