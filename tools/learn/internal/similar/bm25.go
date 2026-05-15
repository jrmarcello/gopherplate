package similar

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
)

// Index identifies one of the FTS5 virtual tables defined by the store schema.
type Index string

// Known FTS5 indexes. Adding a new index requires registering it here so the
// Query helper can validate the caller's choice — we never interpolate a
// caller-controlled string into the SQL.
const (
	IndexSkill   Index = "skill_fts"
	IndexMemory  Index = "memory_fts"
	IndexPattern Index = "pattern_fts"
)

// Match is one ranked hit returned by Query.
//
// SQLite's bm25() function returns more-negative scores for better matches.
// We negate before exposing the score so callers see a conventional
// "higher = more relevant" value, clamped to a non-negative range.
type Match struct {
	Index Index
	Path  string  // present for skill_fts and memory_fts; empty for pattern_fts.
	Score float64 // negated BM25 (>= 0); higher means more relevant.
}

// QueryOpts customizes a BM25 query.
//
// Query must be a pre-sanitized FTS5 MATCH expression — callers should pass
// untrusted input through EscapeMatch first. Limit must be >= 1. MinScore
// filters AFTER ranking and applies to the normalized (negated) score.
type QueryOpts struct {
	Index    Index
	Query    string
	Limit    int
	MinScore float64
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
func Query(ctx context.Context, db *sql.DB, opts QueryOpts) ([]Match, error) {
	if db == nil {
		return nil, learnerr.Runtimef("similar: bm25 query: nil *sql.DB")
	}
	if opts.Limit < 1 {
		return nil, learnerr.Usagef("similar: bm25 query: Limit must be >= 1, got %d", opts.Limit)
	}
	if !isKnownIndex(opts.Index) {
		return nil, learnerr.Usagef("similar: bm25 query: unknown index %q", string(opts.Index))
	}

	// Empty MATCH expression has no defined semantics under FTS5; treat as a
	// no-op rather than letting SQLite raise a parser error.
	if strings.TrimSpace(opts.Query) == "" {
		return []Match{}, nil
	}

	table := string(opts.Index)

	// pattern_fts has no Path column (its UNINDEXED column is kind). For
	// uniformity at the call site, project an empty string in that case.
	var pathExpr string
	if opts.Index == IndexPattern {
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

	rows, queryErr := db.QueryContext(ctx, queryStr, opts.Query, opts.Limit)
	if queryErr != nil {
		return nil, learnerr.Runtimef("similar: bm25 query: %w", queryErr)
	}
	defer func() { _ = rows.Close() }()

	matches := make([]Match, 0, opts.Limit)
	for rows.Next() {
		var (
			path string
			bm25 float64
		)
		if scanErr := rows.Scan(&path, &bm25); scanErr != nil {
			return nil, learnerr.Runtimef("similar: bm25 scan: %w", scanErr)
		}
		// SQLite returns negative BM25 for matches. Negate to flip to a
		// "higher is better" convention; clamp at 0 in case of any future
		// quirk that yields a positive raw score.
		score := -bm25
		if score < 0 {
			score = 0
		}
		if score < opts.MinScore {
			continue
		}
		matches = append(matches, Match{
			Index: opts.Index,
			Path:  path,
			Score: score,
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, learnerr.Runtimef("similar: bm25 rows: %w", rowsErr)
	}

	return matches, nil
}

// EscapeMatch wraps an arbitrary user query in double quotes so that FTS5
// treats it as a phrase, neutralizing the special operators (AND, OR, NOT,
// near, prefix, column-filters, etc.) and any punctuation. Embedded double
// quotes are doubled per the FTS5 string-literal rule. An empty input yields
// an empty string so callers can short-circuit before issuing the query.
func EscapeMatch(s string) string {
	if s == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func isKnownIndex(i Index) bool {
	switch i {
	case IndexSkill, IndexMemory, IndexPattern:
		return true
	default:
		return false
	}
}
