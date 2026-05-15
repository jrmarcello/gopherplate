package pattern

// Score normalizes a Count to [0.0, 1.0]. Algorithm: count / maxCount in the
// same batch, clamped to the valid range.
//
// Special cases:
//   - maxCount <= 0 returns 0.0 (defensive — no batch context, no score).
//   - count <= 0 returns 0.0.
//   - count > maxCount returns 1.0 (clamped — out-of-batch counts are not
//     allowed to exceed the normalized maximum).
func Score(count, maxCount int) float64 {
	if maxCount <= 0 || count <= 0 {
		return 0.0
	}
	if count >= maxCount {
		return 1.0
	}
	return float64(count) / float64(maxCount)
}
