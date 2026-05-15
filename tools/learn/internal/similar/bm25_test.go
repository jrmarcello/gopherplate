package similar

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := EscapeMatch(tc.in); got != tc.want {
				t.Errorf("EscapeMatch(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// openTestStore opens a fresh store backed by a temp file path (modernc.org/sqlite
// supports :memory:, but a temp file is safer for connection-pool semantics).
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "learn.db")
	s, openErr := store.Open(dbPath)
	if openErr != nil {
		t.Fatalf("store.Open: %v", openErr)
	}
	t.Cleanup(func() {
		if closeErr := s.Close(); closeErr != nil {
			t.Errorf("store.Close: %v", closeErr)
		}
	})
	return s
}

// seedSkills inserts three skills with distinct keywords for ranking tests.
func seedSkills(t *testing.T, s *store.Store) {
	t.Helper()
	ctx := context.Background()
	skills := []store.SkillIndexEntry{
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

// dbHandle exposes the underlying *sql.DB. We rely on store.Store exposing DB().
// If the public API does not include this method, we can use the helper below.
func dbHandle(t *testing.T, s *store.Store) interface{} {
	t.Helper()
	return s.DB()
}

func TestQuery_topHitForKeyword(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	seedSkills(t, s)

	ctx := context.Background()
	matches, queryErr := Query(ctx, s.DB(), QueryOpts{
		Index: IndexSkill,
		Query: EscapeMatch("unique-beta-token"),
		Limit: 10,
	})
	if queryErr != nil {
		t.Fatalf("Query: %v", queryErr)
	}
	if len(matches) == 0 {
		t.Fatalf("expected at least one match, got 0")
	}
	if matches[0].Path != "skills/beta/SKILL.md" {
		t.Errorf("top match Path = %q, want skills/beta/SKILL.md", matches[0].Path)
	}
	if matches[0].Index != IndexSkill {
		t.Errorf("top match Index = %q, want %q", matches[0].Index, IndexSkill)
	}
	if matches[0].Score <= 0 {
		t.Errorf("top match Score = %v, want > 0 (negated BM25)", matches[0].Score)
	}
}

// TC-UC-16a determinism: identical queries return identical order and scores.
func TestQuery_deterministic(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	seedSkills(t, s)

	ctx := context.Background()
	opts := QueryOpts{
		Index: IndexSkill,
		Query: EscapeMatch("keyword"),
		Limit: 10,
	}

	first, err1 := Query(ctx, s.DB(), opts)
	if err1 != nil {
		t.Fatalf("Query #1: %v", err1)
	}
	second, err2 := Query(ctx, s.DB(), opts)
	if err2 != nil {
		t.Fatalf("Query #2: %v", err2)
	}

	if len(first) != len(second) {
		t.Fatalf("len mismatch: first=%d second=%d", len(first), len(second))
	}
	for i := range first {
		if first[i].Path != second[i].Path {
			t.Errorf("row[%d] path differs: %q vs %q", i, first[i].Path, second[i].Path)
		}
		if first[i].Score != second[i].Score {
			t.Errorf("row[%d] score differs: %v vs %v", i, first[i].Score, second[i].Score)
		}
	}
}

func TestQuery_emptyIndex(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	// No seed.

	matches, queryErr := Query(context.Background(), s.DB(), QueryOpts{
		Index: IndexSkill,
		Query: EscapeMatch("anything"),
		Limit: 10,
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
	s := openTestStore(t)
	seedSkills(t, s)

	matches, queryErr := Query(context.Background(), s.DB(), QueryOpts{
		Index:    IndexSkill,
		Query:    EscapeMatch("keyword"),
		Limit:    10,
		MinScore: 1e9,
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
	s := openTestStore(t)
	seedSkills(t, s)

	matches, queryErr := Query(context.Background(), s.DB(), QueryOpts{
		Index: IndexSkill,
		Query: EscapeMatch("keyword"),
		Limit: 1,
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
	s := openTestStore(t)
	seedSkills(t, s)

	_, queryErr := Query(context.Background(), s.DB(), QueryOpts{
		Index: IndexSkill,
		Query: EscapeMatch("keyword"),
		Limit: 0,
	})
	if queryErr == nil {
		t.Errorf("expected error for Limit=0, got nil")
	}
}

func TestQuery_invalidIndex(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	_, queryErr := Query(context.Background(), s.DB(), QueryOpts{
		Index: Index("haxx_fts; DROP TABLE skill_fts; --"),
		Query: EscapeMatch("anything"),
		Limit: 10,
	})
	if queryErr == nil {
		t.Errorf("expected error for unknown index, got nil")
	}
}

// suppress unused dbHandle if reflect ever drifts
var _ = dbHandle
