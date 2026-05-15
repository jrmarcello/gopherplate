// Package similar provides string-similarity utilities used by the learn loop:
//
//   - Levenshtein edit distance over UTF-8 rune sequences (Distance,
//     NormalizedDistance).
//   - A thin BM25 wrapper around the SQLite FTS5 virtual tables in
//     internal/store (Query, EscapeMatch).
//
// The package has no dependencies beyond the standard library and
// internal/learnerr for typed error wrapping.
package similar

// Distance returns the Levenshtein edit distance between a and b, measured in
// UTF-8 runes (not bytes). The implementation uses the classic two-row dynamic
// programming algorithm, collapsed to a single row plus two scalars so the
// auxiliary memory is O(min(m, n)) where m, n are the rune lengths.
//
// Edge cases:
//
//   - Distance("", "") == 0.
//   - Distance(s, "") == len([]rune(s)).
//   - Distance("", s) == len([]rune(s)).
//
// The function is symmetric: Distance(a, b) == Distance(b, a).
func Distance(a, b string) int {
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

// NormalizedDistance returns Distance(a, b) divided by the maximum rune length
// of a and b, yielding a value in [0.0, 1.0]. Values closer to 0.0 indicate
// higher similarity. Returns 0.0 when both inputs are empty (vacuously
// identical).
func NormalizedDistance(a, b string) float64 {
	la := runeLen(a)
	lb := runeLen(b)
	maxLen := la
	if lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0.0
	}
	return float64(Distance(a, b)) / float64(maxLen)
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
