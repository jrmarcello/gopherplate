// similar subcommand implementation (REQ-8). Given a target SKILL.md, finds
// the top-K most similar skills from skill_index by combining BM25 ranking
// (FTS5) with Levenshtein normalized edit distance, and emits the result as a
// JSON array on stdout.
//
// Pipeline:
//
//  1. Load config; resolve threshold from --threshold or cfg.SimilarityThreshold.
//  2. Open store.
//  3. Read target SKILL.md, extract frontmatter (title + tags) and body.
//  4. Compose an FTS5 MATCH query from title + tags (or first 20 words of body
//     when no tags), wrapped via similar.EscapeMatch.
//  5. Call similar.Query(IndexSkill, query, topK, threshold).
//  6. Exclude the target itself from results.
//  7. For each remaining match, fetch the candidate's body from skill_index
//     and compute similar.NormalizedDistance for an edit_distance metric.
//  8. Emit JSON array sorted by (-Score, Path) — similar.Query already sorts
//     this way, and we preserve that order while augmenting rows.
package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/similar"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
	"github.com/spf13/cobra"
)

const (
	defaultSimilarTopK  = 5
	bodyFallbackWords   = 20
	defaultDBPathSubcmd = ".claude/learning/db.sqlite"
	defaultConfigPath   = ".claude/learning/config.yml"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newSimilarCmd())
	})
}

// queryFn matches the signature of similar.Query. It is injected through
// similarOpts so threshold-boundary tests can return fabricated matches
// without relying on BM25 scoring exactly hitting a numeric boundary.
type queryFn func(ctx context.Context, db *sql.DB, opts similar.QueryOpts) ([]similar.Match, error)

// similarOpts is the testable input to runSimilar.
//
// Threshold == 0 is a sentinel: when zero, runSimilar resolves the threshold
// from Config.SimilarityThreshold (loaded from ConfigPath unless Config is
// pre-populated). Threshold > 0 always wins over config.
//
// QueryFn defaults to similar.Query; tests inject a fake to control matches.
type similarOpts struct {
	DBPath     string
	ConfigPath string
	SkillPath  string
	TopK       int
	Threshold  float64
	Config     *config.Config
	QueryFn    queryFn
	Stdout     io.Writer
	Stderr     io.Writer
}

// similarRow is one entry in the JSON array emitted on stdout. Field order
// in JSON matches struct field order — keep Path, Score, EditDistance.
type similarRow struct {
	Path         string  `json:"path"`
	Score        float64 `json:"score"`
	EditDistance float64 `json:"edit_distance"`
}

// newSimilarCmd builds the `learn similar` cobra command.
func newSimilarCmd() *cobra.Command {
	var (
		dbPath     string
		configPath string
		skillPath  string
		topK       int
		threshold  float64
	)

	c := &cobra.Command{
		Use:   "similar",
		Short: "Find skills similar to a target SKILL.md (BM25 + edit-distance)",
		Long: `similar reads the target SKILL.md, builds an FTS5 MATCH query from its
title and tags (or the first 20 words of the body when no tags are present),
and emits the top-K most similar skills from skill_index as a JSON array on
stdout. Each row carries the BM25 score (higher = more relevant) and the
Levenshtein normalized edit distance (0.0 = identical, 1.0 = fully different).

The target skill itself is always excluded from the results.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimilar(cmd.Context(), similarOpts{
				DBPath:     dbPath,
				ConfigPath: configPath,
				SkillPath:  skillPath,
				TopK:       topK,
				Threshold:  threshold,
				Stdout:     cmd.OutOrStdout(),
				Stderr:     cmd.ErrOrStderr(),
			})
		},
	}
	c.Flags().StringVar(&dbPath, "db-path", defaultDBPathSubcmd, "Path to the SQLite store")
	c.Flags().StringVar(&configPath, "config", defaultConfigPath, "Path to learning config (for similarity_threshold default)")
	c.Flags().StringVar(&skillPath, "skill", "", "Path to the target SKILL.md (required)")
	c.Flags().IntVar(&topK, "top-k", defaultSimilarTopK, "Maximum number of similar skills to return")
	c.Flags().Float64Var(&threshold, "threshold", 0.0, "BM25 score threshold (>=); 0 means use config.similarity_threshold")
	return c
}

// runSimilar executes the similar subcommand. It validates inputs, loads
// config, opens the store, runs the BM25 query, augments matches with
// edit-distance, and writes a JSON array to stdout.
func runSimilar(ctx context.Context, o similarOpts) error {
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.SkillPath == "" {
		return learnerr.Usagef("similar: --skill must be non-empty")
	}
	if o.DBPath == "" {
		return learnerr.Usagef("similar: --db-path must be non-empty")
	}
	if o.TopK < 1 {
		o.TopK = defaultSimilarTopK
	}
	if o.QueryFn == nil {
		o.QueryFn = similar.Query
	}

	// Resolve threshold: explicit --threshold > 0 wins; otherwise fall back
	// to config.SimilarityThreshold (loaded if not pre-set on opts).
	threshold := o.Threshold
	if threshold == 0.0 {
		cfg := o.Config
		if cfg == nil {
			loaded, loadErr := config.Load(o.ConfigPath)
			if loadErr != nil {
				return learnerr.Runtimef("similar: load config: %w", loadErr)
			}
			cfg = loaded
		}
		threshold = cfg.SimilarityThreshold
	}

	// Read target skill file from disk and extract frontmatter + body.
	target, parseErr := readTargetSkill(o.SkillPath)
	if parseErr != nil {
		return parseErr
	}

	// Compose FTS5 MATCH query — title + tags, or first N words of body when
	// no tags are present. EscapeMatch wraps the result as a phrase literal
	// so FTS5 operators inside the data cannot inject syntax.
	matchExpr := composeMatchQuery(target)

	// Open store.
	st, openErr := store.Open(o.DBPath)
	if openErr != nil {
		return openErr // already *learnerr.RuntimeError
	}
	defer func() { _ = st.Close() }()

	matches, queryErr := o.QueryFn(ctx, st.DB(), similar.QueryOpts{
		Index:    similar.IndexSkill,
		Query:    matchExpr,
		Limit:    o.TopK + 1, // +1 to leave room after excluding the target
		MinScore: threshold,
	})
	if queryErr != nil {
		return learnerr.Runtimef("similar: bm25 query: %w", queryErr)
	}

	rows := make([]similarRow, 0, len(matches))
	for _, m := range matches {
		if m.Path == o.SkillPath {
			continue
		}
		candBody, bodyErr := loadSkillBody(ctx, st.DB(), m.Path)
		if bodyErr != nil {
			return bodyErr
		}
		rows = append(rows, similarRow{
			Path:         m.Path,
			Score:        m.Score,
			EditDistance: similar.NormalizedDistance(target.body, candBody),
		})
		if len(rows) >= o.TopK {
			break
		}
	}

	body, marshalErr := json.MarshalIndent(rows, "", "  ")
	if marshalErr != nil {
		return learnerr.Runtimef("similar: marshal output: %w", marshalErr)
	}
	body = append(body, '\n')
	if _, writeErr := o.Stdout.Write(body); writeErr != nil {
		return learnerr.Runtimef("similar: write stdout: %w", writeErr)
	}
	return nil
}

// targetSkill is the parsed view of the input SKILL.md.
type targetSkill struct {
	path  string
	title string
	tags  []string
	body  string
}

// readTargetSkill reads the target SKILL.md file and extracts frontmatter
// (title, tags) + body using the same extractor as reindex.go.
func readTargetSkill(path string) (targetSkill, error) {
	raw, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return targetSkill{}, learnerr.Usagef("similar: --skill %q: file does not exist", path)
		}
		return targetSkill{}, learnerr.Runtimef("similar: read %q: %w", path, readErr)
	}
	fm, body, splitErr := extractSkillFrontmatter(raw)
	if splitErr != nil {
		return targetSkill{}, learnerr.Usagef("similar: parse %q: %w", path, splitErr)
	}
	return targetSkill{
		path:  path,
		title: fm.Name,
		tags:  fm.Tags,
		body:  strings.TrimPrefix(body, "\n"),
	}, nil
}

// composeMatchQuery builds the FTS5 MATCH expression from the target's title
// and tags. When neither title nor tags are present it falls back to the
// first N words of the body.
//
// Each token is individually escaped as a phrase literal (via EscapeMatch)
// and joined with the FTS5 OR operator so a candidate that matches ANY
// token surfaces — joining with implicit AND would require every token to
// appear in the candidate, which is too restrictive for "similar"
// semantics. Punctuation and FTS5 operators inside any token are
// neutralized by EscapeMatch's phrase-quoting.
func composeMatchQuery(t targetSkill) string {
	var parts []string
	if t.title != "" {
		// Title may be multi-word; treat each word as its own token so a
		// candidate matching one title word still ranks.
		parts = append(parts, strings.Fields(t.title)...)
	}
	if len(t.tags) > 0 {
		parts = append(parts, t.tags...)
	}
	if len(parts) == 0 {
		// Fallback: first N words of body.
		words := strings.Fields(t.body)
		if len(words) > bodyFallbackWords {
			words = words[:bodyFallbackWords]
		}
		parts = words
	}

	escaped := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		escaped = append(escaped, similar.EscapeMatch(p))
	}
	return strings.Join(escaped, " OR ")
}

// loadSkillBody reads the body column for a skill_index row by path. Returns
// empty string when the row is missing (the path came from FTS5 a moment ago
// but a concurrent reindex could have deleted it — we don't fail the call,
// we degrade the edit_distance to "no body comparison possible").
func loadSkillBody(ctx context.Context, db *sql.DB, path string) (string, error) {
	const q = `SELECT body FROM skill_index WHERE path = ?`
	var body string
	scanErr := db.QueryRowContext(ctx, q, path).Scan(&body)
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return "", nil
		}
		return "", learnerr.Runtimef("similar: load body %q: %w", path, scanErr)
	}
	return body, nil
}
