package learn

// Primitives for extracting and scoring repeated patterns from token sequences
// derived from harness signals (transcripts, git history, spec entries). Also
// defines the typed schema for entries written to candidates.jsonl (REQ-2a).

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// patternSep is the canonical token separator used to join tokens in a Signature.
// Using a non-ASCII arrow avoids collisions with most tokens (which may
// legitimately contain spaces, slashes, or dashes). Used by callers (e.g. the
// extract command) so split/join stays consistent without duplication.
const patternSep = " → "

// Kind enumerates pattern categories. New kinds require an explicit case in
// any code that switches on Kind — no "default" fall-through to silent
// behavior.
type patternKind string

const (
	kindToolSequence  patternKind = "tool-sequence"
	kindFileSequence  patternKind = "file-sequence"
	kindCommitPattern patternKind = "commit-pattern"
	kindErrorFix      patternKind = "error-fix"
)

// allPatternKinds is the canonical list — used for validation and iteration.
var allPatternKinds = []patternKind{kindToolSequence, kindFileSequence, kindCommitPattern, kindErrorFix}

// countedNGram is one extracted n-gram signature with its aggregate occurrence
// count across the input sequences.
type countedNGram struct {
	signature string
	count     int
}

// patternCandidate is one line in candidates.jsonl (REQ-2a).
type patternCandidate struct {
	Kind        patternKind `json:"kind"`
	Signature   string      `json:"signature"`
	Frequency   int         `json:"frequency"`
	Contexts    []string    `json:"contexts"`
	Score       float64     `json:"score"`
	FirstSeenAt time.Time   `json:"first_seen_at"`
	LastSeenAt  time.Time   `json:"last_seen_at"`
}

// extractNGrams returns the unique contiguous n-grams of `size` consecutive
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
func extractNGrams(sequences [][]string, size, minFreq int) []countedNGram {
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
			sig := strings.Join(seq[i:i+size], patternSep)
			counts[sig]++
		}
	}

	out := make([]countedNGram, 0, len(counts))
	for sig, n := range counts {
		if n < minFreq {
			continue
		}
		out = append(out, countedNGram{signature: sig, count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].signature < out[j].signature })
	return out
}

// Score normalizes a Count to [0.0, 1.0]. Algorithm: count / maxCount in the
// same batch, clamped to the valid range.
//
// Special cases:
//   - maxCount <= 0 returns 0.0 (defensive — no batch context, no score).
//   - count <= 0 returns 0.0.
//   - count > maxCount returns 1.0 (clamped — out-of-batch counts are not
//     allowed to exceed the normalized maximum).
func scorePattern(count, maxCount int) float64 {
	if maxCount <= 0 || count <= 0 {
		return 0.0
	}
	if count >= maxCount {
		return 1.0
	}
	return float64(count) / float64(maxCount)
}

// kindValid reports whether k is one of allPatternKinds.
func kindValid(k patternKind) bool {
	for _, candidate := range allPatternKinds {
		if candidate == k {
			return true
		}
	}
	return false
}

// Validate returns nil if the patternCandidate is well-formed, or a descriptive error.
//   - Kind must be one of allPatternKinds
//   - Signature non-empty
//   - Frequency >= 1
//   - Score in [0.0, 1.0]
//   - Contexts non-nil (empty slice ok, but nil not)
//   - FirstSeenAt <= LastSeenAt
func (c *patternCandidate) Validate() error {
	if c == nil {
		return errors.New("candidate is nil")
	}
	if !kindValid(c.Kind) {
		return fmt.Errorf("invalid kind %q: must be one of %v", c.Kind, allPatternKinds)
	}
	if c.Signature == "" {
		return errors.New("signature must not be empty")
	}
	if c.Frequency < 1 {
		return fmt.Errorf("frequency must be >= 1, got %d", c.Frequency)
	}
	if c.Score < 0.0 || c.Score > 1.0 {
		return fmt.Errorf("score must be within [0.0, 1.0], got %v", c.Score)
	}
	if c.Contexts == nil {
		return errors.New("contexts must be a non-nil slice (use [] for empty)")
	}
	if c.FirstSeenAt.After(c.LastSeenAt) {
		return fmt.Errorf("first_seen_at %s must be <= last_seen_at %s",
			c.FirstSeenAt.Format(time.RFC3339), c.LastSeenAt.Format(time.RFC3339))
	}
	return nil
}

// Marshal writes a patternCandidate as a single JSONL line (trailing \n).
// Used by `extract` to write candidates.jsonl deterministically. The output
// is byte-stable for identical inputs because encoding/json emits struct
// fields in declaration order and the timestamps are serialized in RFC3339.
func marshalCandidate(c *patternCandidate) ([]byte, error) {
	if c == nil {
		return nil, errors.New("marshal: candidate is nil")
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal candidate: %w", err)
	}
	return append(b, '\n'), nil
}

// unmarshalCandidateStrict parses one JSONL line into a patternCandidate, rejecting unknown
// fields (TC-D-14). Use json.Decoder.DisallowUnknownFields.
//
// The line may include the trailing newline emitted by Marshal; it is
// tolerated. Strict semantics apply only to top-level unknown fields.
func unmarshalCandidateStrict(line []byte) (*patternCandidate, error) {
	trimmed := bytes.TrimRight(line, "\n")
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var c patternCandidate
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("unmarshal candidate: %w", err)
	}
	return &c, nil
}
