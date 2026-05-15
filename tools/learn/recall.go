// recall.go implements the pure retrieval logic for the learning loop
// (REQ-14, REQ-15, REQ-16). It aggregates BM25-ranked matches across the
// skill_fts, memory_fts and pattern_fts indexes, applies score/budget pruning,
// and returns a stable, ranked slice of recallMatch suitable for the
// system-reminder injected by the UserPromptSubmit hook or the manual
// `learn recall` CLI.
package learn

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"
	"unicode"
)

// summaryMaxChars caps the length of an individual recallMatch.Summary. The
// hard cap is the project convention from REQ-14 ("~120 chars of the indexed
// body") and is enforced regardless of the caller's MaxTokens budget — even if
// the budget would allow more, the per-match summary stays compact so the
// system-reminder remains readable.
const summaryMaxChars = 120

// Kind* are the canonical recallMatch.Kind values. They are also the values
// accepted by the CLI `--kind` flag.
const (
	recallKindSkill   = "skill"
	recallKindMemory  = "memory"
	recallKindPattern = "pattern"
)

// recallMatch is one ranked artifact returned by Recall.
type recallMatch struct {
	Kind    string  // "skill" | "memory" | "pattern"
	Path    string  // empty for pattern hits (pattern_fts has no path)
	Score   float64 // normalized BM25 (higher = more relevant), >= 0
	Summary string  // first ~120 chars of the indexed body / signature
}

// recallOptions configures one Recall call.
type recallOptions struct {
	Prompt     string
	TopK       int
	MaxTokens  int
	MinScore   float64
	KindFilter string
	SinceMtime time.Duration

	// Now is injected for deterministic SinceMtime filtering in tests.
	// Defaults to time.Now when nil.
	Now func() time.Time
}

// recallQueryFn is the function signature used to query one FTS index. It
// mirrors Query but is injectable so callers (and tests) can stub the SQL
// layer when needed.
type recallQueryFn func(ctx context.Context, db *sql.DB, opts ftsQueryOpts) ([]similarMatch, error)

// Recall aggregates matches across the configured FTS indexes, ranks them by
// score (descending) with path as a deterministic tie-break, prunes by
// MinScore, attaches a per-match Summary (truncated to summaryMaxChars), and
// finally trims by both TopK and the MaxTokens character budget per REQ-14.
func recallMatches(ctx context.Context, db *sql.DB, opts recallOptions) ([]recallMatch, error) {
	return recallWithQuery(ctx, db, opts, queryFTSIndex)
}

// recallWithQuery is the testable seam — production callers go through
// Recall, tests can stub `q` to bypass the SQLite FTS layer.
func recallWithQuery(ctx context.Context, db *sql.DB, opts recallOptions, q recallQueryFn) ([]recallMatch, error) {
	if db == nil {
		return nil, runtimef("recall: nil *sql.DB")
	}
	if opts.TopK < 1 {
		return nil, usagef("recall: TopK must be >= 1, got %d", opts.TopK)
	}
	if opts.MaxTokens < 1 {
		return nil, usagef("recall: MaxTokens must be >= 1, got %d", opts.MaxTokens)
	}

	escaped := buildMatchExpr(opts.Prompt)
	if escaped == "" {
		return []recallMatch{}, nil
	}

	indexes := indexesForKind(opts.KindFilter)
	if indexes == nil {
		return nil, usagef("recall: invalid KindFilter %q", opts.KindFilter)
	}

	perIndexLimit := opts.TopK
	if perIndexLimit < 5 {
		perIndexLimit = 5
	}

	all := make([]recallMatch, 0, len(indexes)*perIndexLimit)
	for _, idx := range indexes {
		raw, queryErr := q(ctx, db, ftsQueryOpts{
			index:    idx,
			query:    escaped,
			limit:    perIndexLimit,
			minScore: opts.MinScore,
		})
		if queryErr != nil {
			return nil, runtimef("recall: query %s: %w", idx, queryErr)
		}
		for _, r := range raw {
			all = append(all, recallMatch{
				Kind:  kindForIndex(r.index),
				Path:  r.path,
				Score: r.score,
			})
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		return all[i].Path < all[j].Path
	})

	if opts.SinceMtime > 0 {
		filtered := make([]recallMatch, 0, len(all))
		now := opts.Now
		if now == nil {
			now = time.Now
		}
		cutoff := now().UTC().Add(-opts.SinceMtime)
		for _, m := range all {
			keep, mtimeErr := mtimeWithin(ctx, db, m, cutoff)
			if mtimeErr != nil {
				return nil, mtimeErr
			}
			if keep {
				filtered = append(filtered, m)
			}
		}
		all = filtered
	}

	for i := range all {
		summary, summaryErr := loadSummary(ctx, db, all[i])
		if summaryErr != nil {
			return nil, summaryErr
		}
		all[i].Summary = truncate(summary, summaryMaxChars)
	}

	out := make([]recallMatch, 0, opts.TopK)
	budget := opts.MaxTokens
	used := 0
	for _, m := range all {
		if len(out) >= opts.TopK {
			break
		}
		if used+len(m.Summary) > budget {
			break
		}
		out = append(out, m)
		used += len(m.Summary)
	}
	return out, nil
}

func indexesForKind(kind string) []ftsIndex {
	switch kind {
	case "":
		return []ftsIndex{indexSkill, indexMemory, indexPattern}
	case recallKindSkill:
		return []ftsIndex{indexSkill}
	case recallKindMemory:
		return []ftsIndex{indexMemory}
	case recallKindPattern:
		return []ftsIndex{indexPattern}
	default:
		return nil
	}
}

func kindForIndex(idx ftsIndex) string {
	switch idx {
	case indexSkill:
		return recallKindSkill
	case indexMemory:
		return recallKindMemory
	case indexPattern:
		return recallKindPattern
	default:
		return string(idx)
	}
}

func loadSummary(ctx context.Context, db *sql.DB, m recallMatch) (string, error) {
	switch m.Kind {
	case recallKindSkill:
		return readSingleString(ctx, db, `SELECT body FROM skill_index WHERE path = ?`, m.Path)
	case recallKindMemory:
		return readSingleString(ctx, db, `SELECT body FROM memory_index WHERE path = ?`, m.Path)
	case recallKindPattern:
		return "", nil
	default:
		return "", nil
	}
}

func readSingleString(ctx context.Context, db *sql.DB, query string, args ...any) (string, error) {
	var out string
	scanErr := db.QueryRowContext(ctx, query, args...).Scan(&out)
	if scanErr == sql.ErrNoRows {
		return "", nil
	}
	if scanErr != nil {
		return "", runtimef("recall: load summary: %w", scanErr)
	}
	return out, nil
}

func mtimeWithin(ctx context.Context, db *sql.DB, m recallMatch, cutoff time.Time) (bool, error) {
	var (
		query string
		args  []any
	)
	switch m.Kind {
	case recallKindSkill:
		query, args = `SELECT indexed_at FROM skill_index WHERE path = ?`, []any{m.Path}
	case recallKindMemory:
		query, args = `SELECT indexed_at FROM memory_index WHERE path = ?`, []any{m.Path}
	case recallKindPattern:
		return true, nil
	default:
		return true, nil
	}
	raw, readErr := readSingleString(ctx, db, query, args...)
	if readErr != nil {
		return false, readErr
	}
	if raw == "" {
		return false, nil
	}
	ts, parseErr := time.Parse(time.RFC3339Nano, raw)
	if parseErr != nil {
		ts2, parse2Err := time.Parse(time.RFC3339, raw)
		if parse2Err != nil {
			return false, runtimef("recall: parse indexed_at %q: %w", raw, parse2Err)
		}
		ts = ts2
	}
	return !ts.Before(cutoff), nil
}

func buildMatchExpr(prompt string) string {
	tokens := tokenize(prompt)
	if len(tokens) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(tokens))
	for _, t := range tokens {
		safe := strings.ReplaceAll(t, `"`, `""`)
		quoted = append(quoted, `"`+safe+`"`)
	}
	return strings.Join(quoted, " OR ")
}

func tokenize(prompt string) []string {
	var (
		out  []string
		curr strings.Builder
	)
	flush := func() {
		if curr.Len() >= 2 {
			out = append(out, strings.ToLower(curr.String()))
		}
		curr.Reset()
	}
	for _, r := range prompt {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			curr.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
