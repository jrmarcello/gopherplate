package learn

import (
	"context"
	"math"
	"path/filepath"
	"testing"
)

func TestEscapeMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"simple", "foo bar", `"foo bar"`},
		{"embedded quote", `a"b`, `"a""b"`},
		{"operator-looking words", "AND OR NOT", `"AND OR NOT"`},
		{"only a quote", `"`, `""""`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := escapeFTSMatch(tc.in); got != tc.want {
				t.Errorf("escapeFTSMatch(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// openTestStore opens a fresh store backed by a temp file path (modernc.org/sqlite
// supports :memory:, but a temp file is safer for connection-pool semantics).
func openTestStoreForSimilar(t *testing.T) *sqliteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "learn.db")
	s, openErr := openStore(dbPath)
	if openErr != nil {
		t.Fatalf("Open: %v", openErr)
	}
	t.Cleanup(func() {
		if closeErr := s.Close(); closeErr != nil {
			t.Errorf("Close: %v", closeErr)
		}
	})
	return s
}

// seedSkillsForSimilar inserts three skills with distinct keywords for ranking tests.
func seedSkillsForSimilar(t *testing.T, s *sqliteStore) {
	t.Helper()
	ctx := context.Background()
	skills := []skillIndexEntry{
		{
			Path:            "skills/alpha/SKILL.md",
			Title:           "Alpha skill",
			Body:            "alpha keyword unique-alpha-token",
			Tags:            "alpha",
			FrontmatterJSON: "{}",
			IndexedAt:       "2025-01-01T00:00:00Z",
		},
		{
			Path:            "skills/beta/SKILL.md",
			Title:           "Beta skill",
			Body:            "beta keyword unique-beta-token",
			Tags:            "beta",
			FrontmatterJSON: "{}",
			IndexedAt:       "2025-01-01T00:00:00Z",
		},
		{
			Path:            "skills/gamma/SKILL.md",
			Title:           "Gamma skill",
			Body:            "gamma keyword unique-gamma-token",
			Tags:            "gamma",
			FrontmatterJSON: "{}",
			IndexedAt:       "2025-01-01T00:00:00Z",
		},
	}
	for _, sk := range skills {
		if upErr := s.UpsertSkillIndex(ctx, sk); upErr != nil {
			t.Fatalf("UpsertSkillIndex(%s): %v", sk.Path, upErr)
		}
	}
}

func TestQuery_topHitForKeyword(t *testing.T) {
	t.Parallel()
	s := openTestStoreForSimilar(t)
	seedSkillsForSimilar(t, s)

	ctx := context.Background()
	matches, queryErr := queryFTSIndex(ctx, s.DB(), ftsQueryOpts{
		index: indexSkill,
		query: escapeFTSMatch("unique-beta-token"),
		limit: 10,
	})
	if queryErr != nil {
		t.Fatalf("Query: %v", queryErr)
	}
	if len(matches) == 0 {
		t.Fatalf("expected at least one match, got 0")
	}
	if matches[0].path != "skills/beta/SKILL.md" {
		t.Errorf("top match Path = %q, want skills/beta/SKILL.md", matches[0].path)
	}
	if matches[0].index != indexSkill {
		t.Errorf("top match Index = %q, want %q", matches[0].index, indexSkill)
	}
	if matches[0].score <= 0 {
		t.Errorf("top match Score = %v, want > 0 (negated BM25)", matches[0].score)
	}
}

// TC-UC-16a determinism: identical queries return identical order and scores.
func TestQuery_deterministic(t *testing.T) {
	t.Parallel()
	s := openTestStoreForSimilar(t)
	seedSkillsForSimilar(t, s)

	ctx := context.Background()
	opts := ftsQueryOpts{
		index: indexSkill,
		query: escapeFTSMatch("keyword"),
		limit: 10,
	}

	first, err1 := queryFTSIndex(ctx, s.DB(), opts)
	if err1 != nil {
		t.Fatalf("Query #1: %v", err1)
	}
	second, err2 := queryFTSIndex(ctx, s.DB(), opts)
	if err2 != nil {
		t.Fatalf("Query #2: %v", err2)
	}

	if len(first) != len(second) {
		t.Fatalf("len mismatch: first=%d second=%d", len(first), len(second))
	}
	for i := range first {
		if first[i].path != second[i].path {
			t.Errorf("row[%d] path differs: %q vs %q", i, first[i].path, second[i].path)
		}
		if first[i].score != second[i].score {
			t.Errorf("row[%d] score differs: %v vs %v", i, first[i].score, second[i].score)
		}
	}
}

func TestQuery_emptyIndex(t *testing.T) {
	t.Parallel()
	s := openTestStoreForSimilar(t)
	// No seed.

	matches, queryErr := queryFTSIndex(context.Background(), s.DB(), ftsQueryOpts{
		index: indexSkill,
		query: escapeFTSMatch("anything"),
		limit: 10,
	})
	if queryErr != nil {
		t.Fatalf("Query: %v", queryErr)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestQuery_minScoreFiltersAll(t *testing.T) {
	t.Parallel()
	s := openTestStoreForSimilar(t)
	seedSkillsForSimilar(t, s)

	matches, queryErr := queryFTSIndex(context.Background(), s.DB(), ftsQueryOpts{
		index:    indexSkill,
		query:    escapeFTSMatch("keyword"),
		limit:    10,
		minScore: 1e9,
	})
	if queryErr != nil {
		t.Fatalf("Query: %v", queryErr)
	}
	if len(matches) != 0 {
		t.Errorf("expected MinScore to filter everything, got %d", len(matches))
	}
}

func TestQuery_limitBoundary(t *testing.T) {
	t.Parallel()
	s := openTestStoreForSimilar(t)
	seedSkillsForSimilar(t, s)

	matches, queryErr := queryFTSIndex(context.Background(), s.DB(), ftsQueryOpts{
		index: indexSkill,
		query: escapeFTSMatch("keyword"),
		limit: 1,
	})
	if queryErr != nil {
		t.Fatalf("Query: %v", queryErr)
	}
	if len(matches) != 1 {
		t.Errorf("expected exactly 1 match with Limit=1, got %d", len(matches))
	}
}

func TestQuery_invalidLimit(t *testing.T) {
	t.Parallel()
	s := openTestStoreForSimilar(t)
	seedSkillsForSimilar(t, s)

	_, queryErr := queryFTSIndex(context.Background(), s.DB(), ftsQueryOpts{
		index: indexSkill,
		query: escapeFTSMatch("keyword"),
		limit: 0,
	})
	if queryErr == nil {
		t.Errorf("expected error for Limit=0, got nil")
	}
}

func TestQuery_invalidIndex(t *testing.T) {
	t.Parallel()
	s := openTestStoreForSimilar(t)

	_, queryErr := queryFTSIndex(context.Background(), s.DB(), ftsQueryOpts{
		index: ftsIndex("haxx_fts; DROP TABLE skill_fts; --"),
		query: escapeFTSMatch("anything"),
		limit: 10,
	})
	if queryErr == nil {
		t.Errorf("expected error for unknown index, got nil")
	}
}

// TC-D-09: classic Levenshtein cases plus rune-correctness.
func TestDistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"kitten/sitting (canonical)", "kitten", "sitting", 3},
		{"equal strings", "abcdef", "abcdef", 0},
		{"both empty", "", "", 0},
		{"a empty", "", "abc", 3},
		{"b empty", "abc", "", 3},
		{"single substitution", "abc", "abd", 1},
		{"single insertion", "abc", "abcd", 1},
		{"single deletion", "abcd", "abc", 1},
		{"reversed equal-length", "abc", "cba", 2},
		{"rune precomposed equal", "café", "café", 0},
		{"rune diff single edit", "café", "cafe", 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := levenshteinDistance(tc.a, tc.b); got != tc.want {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestDistance_symmetric(t *testing.T) {
	t.Parallel()
	pairs := [][2]string{
		{"kitten", "sitting"},
		{"café", "cafe"},
		{"abc", "xyz"},
	}
	for _, p := range pairs {
		if levenshteinDistance(p[0], p[1]) != levenshteinDistance(p[1], p[0]) {
			t.Errorf("levenshteinDistance not symmetric for (%q,%q)", p[0], p[1])
		}
	}
}

func TestNormalizedDistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b string
		want float64
	}{
		{"both empty", "", "", 0.0},
		{"equal", "abc", "abc", 0.0},
		{"single replace", "a", "b", 1.0},
		{"kitten/sitting", "kitten", "sitting", 3.0 / 7.0},
		{"empty vs abc", "", "abc", 1.0},
	}

	const eps = 1e-9
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizedLevenshteinDistance(tc.a, tc.b)
			if math.Abs(got-tc.want) > eps {
				t.Errorf("normalizedLevenshteinDistance(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestNormalizedDistance_bounds(t *testing.T) {
	t.Parallel()
	cases := [][2]string{
		{"", ""},
		{"hello", "world"},
		{"abc", "abcd"},
		{"a", "b"},
		{"kitten", "sitting"},
	}
	for _, c := range cases {
		got := normalizedLevenshteinDistance(c[0], c[1])
		if got < 0.0 || got > 1.0 {
			t.Errorf("normalizedLevenshteinDistance(%q,%q)=%v out of [0,1]", c[0], c[1], got)
		}
	}
}
