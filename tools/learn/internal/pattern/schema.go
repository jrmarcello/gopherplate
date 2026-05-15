// Package pattern provides primitives for extracting and scoring repeated
// patterns from token sequences derived from harness signals (transcripts,
// git history, spec entries). It also defines the typed schema for entries
// written to candidates.jsonl (REQ-2a).
package pattern

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Kind enumerates pattern categories. New kinds require an explicit case in
// any code that switches on Kind — no "default" fall-through to silent
// behavior.
type Kind string

const (
	KindToolSequence  Kind = "tool-sequence"
	KindFileSequence  Kind = "file-sequence"
	KindCommitPattern Kind = "commit-pattern"
	KindErrorFix      Kind = "error-fix"
)

// AllKinds is the canonical list — used for validation and iteration.
var AllKinds = []Kind{KindToolSequence, KindFileSequence, KindCommitPattern, KindErrorFix}

// Candidate is one line in candidates.jsonl (REQ-2a).
type Candidate struct {
	Kind        Kind      `json:"kind"`
	Signature   string    `json:"signature"`
	Frequency   int       `json:"frequency"`
	Contexts    []string  `json:"contexts"`
	Score       float64   `json:"score"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// kindValid reports whether k is one of AllKinds.
func kindValid(k Kind) bool {
	for _, candidate := range AllKinds {
		if candidate == k {
			return true
		}
	}
	return false
}

// Validate returns nil if the Candidate is well-formed, or a descriptive error.
//   - Kind must be one of AllKinds
//   - Signature non-empty
//   - Frequency >= 1
//   - Score in [0.0, 1.0]
//   - Contexts non-nil (empty slice ok, but nil not)
//   - FirstSeenAt <= LastSeenAt
func (c *Candidate) Validate() error {
	if c == nil {
		return errors.New("candidate is nil")
	}
	if !kindValid(c.Kind) {
		return fmt.Errorf("invalid kind %q: must be one of %v", c.Kind, AllKinds)
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

// Marshal writes a Candidate as a single JSONL line (trailing \n).
// Used by `extract` to write candidates.jsonl deterministically. The output
// is byte-stable for identical inputs because encoding/json emits struct
// fields in declaration order and the timestamps are serialized in RFC3339.
func Marshal(c *Candidate) ([]byte, error) {
	if c == nil {
		return nil, errors.New("marshal: candidate is nil")
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal candidate: %w", err)
	}
	return append(b, '\n'), nil
}

// UnmarshalStrict parses one JSONL line into a Candidate, rejecting unknown
// fields (TC-D-14). Use json.Decoder.DisallowUnknownFields.
//
// The line may include the trailing newline emitted by Marshal; it is
// tolerated. Strict semantics apply only to top-level unknown fields.
func UnmarshalStrict(line []byte) (*Candidate, error) {
	trimmed := bytes.TrimRight(line, "\n")
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var c Candidate
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("unmarshal candidate: %w", err)
	}
	return &c, nil
}
