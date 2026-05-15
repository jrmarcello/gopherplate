// String-similarity utilities used by the learn loop:
//
//   - Levenshtein edit distance over UTF-8 rune sequences (levenshteinDistance,
//     normalizedLevenshteinDistance).
//   - A thin BM25 wrapper around the SQLite FTS5 virtual tables in the
//     store (Query, escapeFTSMatch).
//
// The functions have no dependencies beyond the standard library and the
// typed error wrappers in this package.

package learn

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Index identifies one of the FTS5 virtual tables defined by the store schema.
type ftsIndex string

// Known FTS5 indexes. Adding a new index requires registering it here so the
// Query helper can validate the caller's choice — we never interpolate a
// caller-controlled string into the SQL.
const (
	indexSkill   ftsIndex = "skill_fts"
	indexMemory  ftsIndex = "memory_fts"
	indexPattern ftsIndex = "pattern_fts"
)

// similarMatch is one ranked hit returned by Query.
//
// SQLite's bm25() function returns more-negative scores for better matches.
// We negate before exposing the score so callers see a conventional
// "higher = more relevant" value, clamped to a non-negative range.
type similarMatch struct {
	index ftsIndex
	path  string  // present for skill_fts and memory_fts; empty for pattern_fts.
	score float64 // negated BM25 (>= 0); higher means more relevant.
}

// ftsQueryOpts customizes a BM25 query.
//
// Query must be a pre-sanitized FTS5 MATCH expression — callers should pass
// untrusted input through escapeFTSMatch first. Limit must be >= 1. MinScore
// filters AFTER ranking and applies to the normalized (negated) score.
type ftsQueryOpts struct {
	index    ftsIndex
	query    string
	limit    int
	minScore float64
}

// Query executes a BM25-ranked FTS5 MATCH against the configured index and
// returns results sorted by relevance descending. Ties on score are broken by
// Path ascending for determinism (TC-UC-16a).
//
// The function uses parameterized statements for the MATCH value and the
// LIMIT. The table name IS interpolated via fmt.Sprintf, but the value is
// validated against the fixed allow-list `isKnownIndex` (skill_fts /
// memory_fts / pattern_fts) before interpolation — that allow-list is the
// real defense against SQL injection here, not the absence of interpolation.
func queryFTSIndex(ctx context.Context, db *sql.DB, opts ftsQueryOpts) ([]similarMatch, error) {
	if db == nil {
		return nil, runtimef("similar: bm25 query: nil *sql.DB")
	}
	if opts.limit < 1 {
		return nil, usagef("similar: bm25 query: Limit must be >= 1, got %d", opts.limit)
	}
	if !isKnownIndex(opts.index) {
		return nil, usagef("similar: bm25 query: unknown index %q", string(opts.index))
	}

	// Empty MATCH expression has no defined semantics under FTS5; treat as a
	// no-op rather than letting SQLite raise a parser error.
	if strings.TrimSpace(opts.query) == "" {
		return []similarMatch{}, nil
	}

	table := string(opts.index)

	// pattern_fts has no Path column (its UNINDEXED column is kind). For
	// uniformity at the call site, project an empty string in that case.
	var pathExpr string
	if opts.index == indexPattern {
		pathExpr = "''"
	} else {
		pathExpr = "path"
	}

	// Order by bm25() ascending (most negative first = best match), then by
	// Path ascending for tie-break determinism. table is allowlisted above
	// via isKnownIndex; pathExpr is either "path" or "''" (constants).
	queryStr := fmt.Sprintf( //nolint:gosec // table is allowlisted (isKnownIndex); no caller input reaches the SQL
		`SELECT %s, bm25(%s) FROM %s WHERE %s MATCH ? ORDER BY bm25(%s) ASC, %s ASC LIMIT ?`,
		pathExpr, table, table, table, table, pathExpr,
	)

	rows, queryErr := db.QueryContext(ctx, queryStr, opts.query, opts.limit)
	if queryErr != nil {
		return nil, runtimef("similar: bm25 query: %w", queryErr)
	}
	defer func() { _ = rows.Close() }()

	matches := make([]similarMatch, 0, opts.limit)
	for rows.Next() {
		var (
			path string
			bm25 float64
		)
		if scanErr := rows.Scan(&path, &bm25); scanErr != nil {
			return nil, runtimef("similar: bm25 scan: %w", scanErr)
		}
		// SQLite returns negative BM25 for matches. Negate to flip to a
		// "higher is better" convention; clamp at 0 in case of any future
		// quirk that yields a positive raw score.
		score := -bm25
		if score < 0 {
			score = 0
		}
		if score < opts.minScore {
			continue
		}
		matches = append(matches, similarMatch{
			index: opts.index,
			path:  path,
			score: score,
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, runtimef("similar: bm25 rows: %w", rowsErr)
	}

	return matches, nil
}

// escapeFTSMatch wraps an arbitrary user query in double quotes so that FTS5
// treats it as a phrase, neutralizing the special operators (AND, OR, NOT,
// near, prefix, column-filters, etc.) and any punctuation. Embedded double
// quotes are doubled per the FTS5 string-literal rule. An empty input yields
// an empty string so callers can short-circuit before issuing the query.
func escapeFTSMatch(s string) string {
	if s == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func isKnownIndex(i ftsIndex) bool {
	switch i {
	case indexSkill, indexMemory, indexPattern:
		return true
	default:
		return false
	}
}

// levenshteinDistance returns the Levenshtein edit distance between a and b, measured in
// UTF-8 runes (not bytes). The implementation uses the classic two-row dynamic
// programming algorithm, collapsed to a single row plus two scalars so the
// auxiliary memory is O(min(m, n)) where m, n are the rune lengths.
//
// Edge cases:
//
//   - levenshteinDistance("", "") == 0.
//   - levenshteinDistance(s, "") == len([]rune(s)).
//   - levenshteinDistance("", s) == len([]rune(s)).
//
// The function is symmetric: levenshteinDistance(a, b) == levenshteinDistance(b, a).
func levenshteinDistance(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)

	// Make rb the shorter sequence so the working row is O(min(m, n)).
	if len(ra) < len(rb) {
		ra, rb = rb, ra
	}

	m := len(ra)
	n := len(rb)

	if n == 0 {
		return m
	}

	// prev[j] = edit distance between ra[:i-1] and rb[:j].
	prev := make([]int, n+1)
	for j := 0; j <= n; j++ {
		prev[j] = j
	}

	for i := 1; i <= m; i++ {
		// curr[0] for this row is i (cost of deleting i chars from ra).
		prevDiag := prev[0]
		prev[0] = i

		for j := 1; j <= n; j++ {
			// Save prev[j] before overwrite so the next iteration sees the
			// pre-update diagonal value.
			save := prev[j]

			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}

			// Standard recurrence:
			//   delete:    prev[j]  + 1   (drop ra[i-1])
			//   insert:    prev[j-1]+ 1   (insert rb[j-1])
			//   replace:   prevDiag + cost
			del := prev[j] + 1
			ins := prev[j-1] + 1
			sub := prevDiag + cost

			prev[j] = minInt(del, minInt(ins, sub))
			prevDiag = save
		}
	}

	return prev[n]
}

// normalizedLevenshteinDistance returns levenshteinDistance(a, b) divided by the maximum rune length
// of a and b, yielding a value in [0.0, 1.0]. Values closer to 0.0 indicate
// higher similarity. Returns 0.0 when both inputs are empty (vacuously
// identical).
func normalizedLevenshteinDistance(a, b string) float64 {
	la := runeLen(a)
	lb := runeLen(b)
	maxLen := la
	if lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0.0
	}
	return float64(levenshteinDistance(a, b)) / float64(maxLen)
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
