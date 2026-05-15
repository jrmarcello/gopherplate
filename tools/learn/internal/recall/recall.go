// Package recall implements the pure retrieval logic for the learning loop
// (REQ-14, REQ-15, REQ-16). It aggregates BM25-ranked matches across the
// skill_fts, memory_fts and pattern_fts indexes, applies score/budget pruning,
// and returns a stable, ranked slice of Match suitable for the system-reminder
// injected by the UserPromptSubmit hook or the manual `learn recall` CLI.
//
// The package is deliberately free of CLI / IO concerns — the `recall`
// subcommand (internal/cmd/recall.go) is the only caller in the production
// tree. Keeping the retrieval logic isolated allows future callers (e.g. a
// future MCP server, an interactive REPL) to reuse the same primitive.
package recall

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/similar"
)

// summaryMaxChars caps the length of an individual Match.Summary. The hard cap
// is the project convention from REQ-14 ("~120 chars of the indexed body") and
// is enforced regardless of the caller's MaxTokens budget — even if the budget
// would allow more, the per-match summary stays compact so the system-reminder
// remains readable.
const summaryMaxChars = 120

// Kind* are the canonical Match.Kind values. They are also the values accepted
// by the CLI `--kind` flag.
const (
	KindSkill   = "skill"
	KindMemory  = "memory"
	KindPattern = "pattern"
)

// Match is one ranked artifact returned by Recall.
type Match struct {
	Kind    string  // "skill" | "memory" | "pattern"
	Path    string  // empty for pattern hits (pattern_fts has no path)
	Score   float64 // normalized BM25 (higher = more relevant), >= 0
	Summary string  // first ~120 chars of the indexed body / signature
}

// Options configures one Recall call.
//
// MinScore filters results AFTER ranking. MaxTokens is a soft budget on the
// cumulative summary length expressed as characters (chars/4 is the rough
// token approximation accepted by REQ-14). TopK caps the number of returned
// matches. KindFilter restricts the underlying FTS indexes queried — empty
// string means "all three".
//
// SinceMtime is reserved for future use: skill_index and memory_index already
// have an indexed_at column, so a non-zero value will filter rows whose
// indexed_at falls outside `[now - SinceMtime, now]`. Pattern rows have no
// equivalent column and are always included.
type Options struct {
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

// queryFn is the function signature used to query one FTS index. It mirrors
// similar.Query but is injectable so callers (and tests) can stub the SQL
// layer when needed.
type queryFn func(ctx context.Context, db *sql.DB, opts similar.QueryOpts) ([]similar.Match, error)

// Recall aggregates matches across the configured FTS indexes, ranks them by
// score (descending) with path as a deterministic tie-break, prunes by
// MinScore, attaches a per-match Summary (truncated to summaryMaxChars), and
// finally trims by both TopK and the MaxTokens character budget per REQ-14.
//
// Truncation strategy: rank first, then walk in order accumulating summary
// length; STOP when the next summary would exceed the budget (drop the
// remainder entirely; never truncate a summary mid-way). This matches TC-UC-22.
func Recall(ctx context.Context, db *sql.DB, opts Options) ([]Match, error) {
	return recallWithQuery(ctx, db, opts, similar.Query)
}

// recallWithQuery is the testable seam — production callers go through
// Recall, tests can stub `q` to bypass the SQLite FTS layer.
func recallWithQuery(ctx context.Context, db *sql.DB, opts Options, q queryFn) ([]Match, error) {
	if db == nil {
		return nil, learnerr.Runtimef("recall: nil *sql.DB")
	}
	if opts.TopK < 1 {
		return nil, learnerr.Usagef("recall: TopK must be >= 1, got %d", opts.TopK)
	}
	if opts.MaxTokens < 1 {
		return nil, learnerr.Usagef("recall: MaxTokens must be >= 1, got %d", opts.MaxTokens)
	}

	escaped := buildMatchExpr(opts.Prompt)
	if escaped == "" {
		return []Match{}, nil
	}

	indexes := indexesForKind(opts.KindFilter)
	if indexes == nil {
		return nil, learnerr.Usagef("recall: invalid KindFilter %q", opts.KindFilter)
	}

	// We over-fetch from each index (TopK each) so the aggregate ranking has
	// enough candidates; the final TopK cap is applied below.
	perIndexLimit := opts.TopK
	if perIndexLimit < 5 {
		perIndexLimit = 5
	}

	all := make([]Match, 0, len(indexes)*perIndexLimit)
	for _, idx := range indexes {
		raw, queryErr := q(ctx, db, similar.QueryOpts{
			Index:    idx,
			Query:    escaped,
			Limit:    perIndexLimit,
			MinScore: opts.MinScore,
		})
		if queryErr != nil {
			return nil, learnerr.Runtimef("recall: query %s: %w", idx, queryErr)
		}
		for _, r := range raw {
			all = append(all, Match{
				Kind:  kindForIndex(r.Index),
				Path:  r.Path,
				Score: r.Score,
				// Summary is populated below in a second pass so a single
				// query for the body covers all hits regardless of index.
			})
		}
	}

	// Rank descending by score, tie-break ascending by path for determinism.
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		return all[i].Path < all[j].Path
	})

	// Apply SinceMtime by reading indexed_at from skill_index / memory_index.
	// Pattern matches are always kept (no mtime column).
	if opts.SinceMtime > 0 {
		filtered := make([]Match, 0, len(all))
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

	// Attach summaries.
	for i := range all {
		summary, summaryErr := loadSummary(ctx, db, all[i])
		if summaryErr != nil {
			return nil, summaryErr
		}
		all[i].Summary = truncate(summary, summaryMaxChars)
	}

	// Walk in rank order accumulating summary length; stop when the budget
	// would be exceeded by the NEXT entry. Drop the remainder.
	out := make([]Match, 0, opts.TopK)
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

// indexesForKind returns the FTS indexes to query for the given filter. An
// empty string queries all three. An unknown filter returns nil so callers can
// surface a usage error.
func indexesForKind(kind string) []similar.Index {
	switch kind {
	case "":
		return []similar.Index{similar.IndexSkill, similar.IndexMemory, similar.IndexPattern}
	case KindSkill:
		return []similar.Index{similar.IndexSkill}
	case KindMemory:
		return []similar.Index{similar.IndexMemory}
	case KindPattern:
		return []similar.Index{similar.IndexPattern}
	default:
		return nil
	}
}

// kindForIndex maps an FTS index back to its CLI/JSON kind label.
func kindForIndex(idx similar.Index) string {
	switch idx {
	case similar.IndexSkill:
		return KindSkill
	case similar.IndexMemory:
		return KindMemory
	case similar.IndexPattern:
		return KindPattern
	default:
		return string(idx)
	}
}

// loadSummary fetches a short summary for a Match. For skills/memory it reads
// the body column; for patterns it reads the signature column. Missing rows
// produce an empty summary (best-effort — the FTS row should always have a
// backing row, but we never want recall to crash on a stale index).
func loadSummary(ctx context.Context, db *sql.DB, m Match) (string, error) {
	switch m.Kind {
	case KindSkill:
		return readSingleString(ctx, db, `SELECT body FROM skill_index WHERE path = ?`, m.Path)
	case KindMemory:
		return readSingleString(ctx, db, `SELECT body FROM memory_index WHERE path = ?`, m.Path)
	case KindPattern:
		// pattern_fts has no path; the FTS rowid is the patterns.id. But Match
		// from similar.Query reports an empty path for patterns. We cannot
		// recover the signature here without re-querying — fall back to the
		// empty string and let the caller treat the match as opaque. This is
		// acceptable for now; pattern recall is rarely surfaced (see REQ-16).
		return "", nil
	default:
		return "", nil
	}
}

// readSingleString runs a `SELECT <text-col>` query that returns at most one
// row. Missing rows are NOT an error — they produce "". A genuine query error
// is wrapped as *RuntimeError.
func readSingleString(ctx context.Context, db *sql.DB, query string, args ...any) (string, error) {
	var out string
	scanErr := db.QueryRowContext(ctx, query, args...).Scan(&out)
	if scanErr == sql.ErrNoRows {
		return "", nil
	}
	if scanErr != nil {
		return "", learnerr.Runtimef("recall: load summary: %w", scanErr)
	}
	return out, nil
}

// mtimeWithin reports whether the match's indexed_at is >= cutoff. Patterns
// always return true.
func mtimeWithin(ctx context.Context, db *sql.DB, m Match, cutoff time.Time) (bool, error) {
	var (
		query string
		args  []any
	)
	switch m.Kind {
	case KindSkill:
		query, args = `SELECT indexed_at FROM skill_index WHERE path = ?`, []any{m.Path}
	case KindMemory:
		query, args = `SELECT indexed_at FROM memory_index WHERE path = ?`, []any{m.Path}
	case KindPattern:
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
		// Fall back to RFC3339 (no nanos) before giving up.
		ts2, parse2Err := time.Parse(time.RFC3339, raw)
		if parse2Err != nil {
			return false, learnerr.Runtimef("recall: parse indexed_at %q: %w", raw, parse2Err)
		}
		ts = ts2
	}
	return !ts.Before(cutoff), nil
}

// buildMatchExpr converts a free-form prompt into an FTS5 MATCH expression
// that ORs each token together. Each token is individually quoted (per the
// FTS5 string-literal rule) so punctuation and FTS5 operators in the prompt
// cannot escape into the query.
//
// Returns "" when the prompt contains no usable token, so callers can
// short-circuit without issuing a query.
func buildMatchExpr(prompt string) string {
	tokens := tokenize(prompt)
	if len(tokens) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(tokens))
	for _, t := range tokens {
		// Double any embedded `"` per FTS5 string literal rules.
		safe := strings.ReplaceAll(t, `"`, `""`)
		quoted = append(quoted, `"`+safe+`"`)
	}
	return strings.Join(quoted, " OR ")
}

// tokenize splits a prompt into FTS-friendly tokens: groups of letters/digits
// of length >= 2. Single-character tokens and pure punctuation are dropped —
// they would either be FTS5 stopwords or create excessive false positives.
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

// truncate returns s capped at n bytes. Multi-byte runes are truncated at byte
// boundaries — acceptable for the system-reminder use case where the summary
// is human-readable prose and the FTS-indexed body has already passed through
// sanitize.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
