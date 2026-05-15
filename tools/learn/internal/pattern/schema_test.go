package pattern

import (
	"strings"
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return ts
}

func validCandidate(t *testing.T) *Candidate {
	t.Helper()
	return &Candidate{
		Kind:        KindToolSequence,
		Signature:   "a → b",
		Frequency:   3,
		Contexts:    []string{},
		Score:       0.5,
		FirstSeenAt: mustTime(t, "2026-05-01T00:00:00Z"),
		LastSeenAt:  mustTime(t, "2026-05-10T00:00:00Z"),
	}
}

func TestCandidate_Validate_HappyPath(t *testing.T) {
	c := validCandidate(t)
	if err := c.Validate(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestCandidate_Validate_RejectsInvalidKind(t *testing.T) {
	c := validCandidate(t)
	c.Kind = Kind("bogus")
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("expected kind error, got %v", err)
	}
}

func TestCandidate_Validate_RejectsEmptySignature(t *testing.T) {
	c := validCandidate(t)
	c.Signature = ""
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("expected signature error, got %v", err)
	}
}

func TestCandidate_Validate_RejectsFrequencyZero(t *testing.T) {
	c := validCandidate(t)
	c.Frequency = 0
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "frequency") {
		t.Fatalf("expected frequency error, got %v", err)
	}
}

func TestCandidate_Validate_RejectsScoreAboveOne(t *testing.T) {
	c := validCandidate(t)
	c.Score = 1.5
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "score") {
		t.Fatalf("expected score error, got %v", err)
	}
}

func TestCandidate_Validate_RejectsScoreBelowZero(t *testing.T) {
	c := validCandidate(t)
	c.Score = -0.1
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "score") {
		t.Fatalf("expected score error, got %v", err)
	}
}

func TestCandidate_Validate_RejectsNilContexts(t *testing.T) {
	c := validCandidate(t)
	c.Contexts = nil
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "contexts") {
		t.Fatalf("expected contexts error, got %v", err)
	}
}

func TestCandidate_Validate_RejectsFirstSeenAfterLastSeen(t *testing.T) {
	c := validCandidate(t)
	c.FirstSeenAt = mustTime(t, "2026-05-20T00:00:00Z")
	c.LastSeenAt = mustTime(t, "2026-05-10T00:00:00Z")
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "first_seen_at") {
		t.Fatalf("expected first_seen_at error, got %v", err)
	}
}

func TestMarshal_ProducesTrailingNewline(t *testing.T) {
	c := validCandidate(t)
	b, err := Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatalf("expected trailing newline, got %q", b)
	}
}

func TestMarshal_DeterministicForSameInput(t *testing.T) {
	c1 := validCandidate(t)
	c2 := validCandidate(t)
	b1, err1 := Marshal(c1)
	b2, err2 := Marshal(c2)
	if err1 != nil || err2 != nil {
		t.Fatalf("marshal errors: %v %v", err1, err2)
	}
	if string(b1) != string(b2) {
		t.Fatalf("marshal not deterministic:\n%s\nvs\n%s", b1, b2)
	}
}

// TC-D-14: extra unknown field => UnmarshalStrict errors.
func TestUnmarshalStrict_RejectsUnknownField(t *testing.T) {
	line := []byte(`{"kind":"tool-sequence","signature":"a → b","frequency":1,"contexts":[],"score":0.1,"first_seen_at":"2026-05-01T00:00:00Z","last_seen_at":"2026-05-02T00:00:00Z","extra":"hi"}`)
	_, err := UnmarshalStrict(line)
	if err == nil || !strings.Contains(err.Error(), "extra") {
		t.Fatalf("expected unknown field error citing 'extra', got %v", err)
	}
}

// TC-D-14: missing kind => Candidate parses, Validate fails.
func TestUnmarshalStrict_MissingKindParsesButFailsValidation(t *testing.T) {
	line := []byte(`{"signature":"a → b","frequency":1,"contexts":[],"score":0.1,"first_seen_at":"2026-05-01T00:00:00Z","last_seen_at":"2026-05-02T00:00:00Z"}`)
	c, err := UnmarshalStrict(line)
	if err != nil {
		t.Fatalf("unmarshal err: %v", err)
	}
	if c.Kind != "" {
		t.Fatalf("expected empty Kind, got %q", c.Kind)
	}
	if vErr := c.Validate(); vErr == nil {
		t.Fatalf("expected Validate to reject empty kind")
	}
}

func TestUnmarshalStrict_RoundTrip(t *testing.T) {
	c := validCandidate(t)
	b, err := Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := UnmarshalStrict(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Kind != c.Kind || got.Signature != c.Signature || got.Frequency != c.Frequency || got.Score != c.Score {
		t.Fatalf("roundtrip mismatch: got %+v want %+v", got, c)
	}
}
