package similar

import (
	"math"
	"testing"
)

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
			if got := Distance(tc.a, tc.b); got != tc.want {
				t.Errorf("Distance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
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
		if Distance(p[0], p[1]) != Distance(p[1], p[0]) {
			t.Errorf("Distance not symmetric for (%q,%q)", p[0], p[1])
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
			got := NormalizedDistance(tc.a, tc.b)
			if math.Abs(got-tc.want) > eps {
				t.Errorf("NormalizedDistance(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
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
		got := NormalizedDistance(c[0], c[1])
		if got < 0.0 || got > 1.0 {
			t.Errorf("NormalizedDistance(%q,%q)=%v out of [0,1]", c[0], c[1], got)
		}
	}
}
