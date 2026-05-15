package pattern

import (
	"sort"
	"strings"
)

// Sep is the canonical token separator used to join tokens in a Signature.
// Using a non-ASCII arrow avoids collisions with most tokens (which may
// legitimately contain spaces, slashes, or dashes). Exported so callers
// (e.g. extract command) can split/join consistently without duplication.
const Sep = " → "

// signatureSeparator is the internal alias retained for back-compat within
// this package; new code should reference Sep.
const signatureSeparator = Sep

// Counted is one extracted n-gram signature with its aggregate occurrence
// count across the input sequences.
type Counted struct {
	Signature string
	Count     int
}

// ExtractNGrams returns the unique contiguous n-grams of `size` consecutive
// tokens that appear AT LEAST `minFreq` times across all input sequences,
// preserving the canonical order. Output is sorted by signature ascending
// (deterministic — REQ-2 requires byte-identical re-runs).
//
// Each input is a slice of tokens (already normalized by the caller —
// pattern doesn't care what the tokens are; ingest packages normalize
// transcripts/git/spec entries into token slices).
//
// Signatures: tokens joined by " → " (U+2192).
//
// Behavior:
//   - size < 1 returns empty.
//   - minFreq < 1 is treated as 1.
//   - Sequences shorter than size contribute nothing.
//   - Counts aggregate across sequences.
func ExtractNGrams(sequences [][]string, size, minFreq int) []Counted {
	if size < 1 {
		return nil
	}
	if minFreq < 1 {
		minFreq = 1
	}

	counts := make(map[string]int)
	for _, seq := range sequences {
		if len(seq) < size {
			continue
		}
		for i := 0; i+size <= len(seq); i++ {
			sig := strings.Join(seq[i:i+size], signatureSeparator)
			counts[sig]++
		}
	}

	out := make([]Counted, 0, len(counts))
	for sig, n := range counts {
		if n < minFreq {
			continue
		}
		out = append(out, Counted{Signature: sig, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Signature < out[j].Signature })
	return out
}
