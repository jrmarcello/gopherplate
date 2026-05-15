package learn

import (
	"bytes"
	"sort"
	"strings"
	"testing"
	"time"
)

// TC-D-01: minFreq=3, three identical 2-grams across input => emitted with Count=3.
func TestExtractNGrams_EmitsWhenCountEqualsMinFreq(t *testing.T) {
	seqs := [][]string{
		{"a", "b", "c"},
		{"a", "b", "d"},
		{"x", "a", "b"},
	}
	got := extractNGrams(seqs, 2, 3)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(got), got)
	}
	if got[0].signature != "a → b" || got[0].count != 3 {
		t.Fatalf("expected a → b count=3, got %+v", got[0])
	}
}

// TC-D-02: minFreq=3, only two occurrences => NOT emitted.
func TestExtractNGrams_OmitsWhenCountBelowMinFreq(t *testing.T) {
	seqs := [][]string{
		{"a", "b", "c"},
		{"a", "b", "d"},
	}
	got := extractNGrams(seqs, 2, 3)
	if len(got) != 0 {
		t.Fatalf("expected 0 results, got %+v", got)
	}
}

// TC-D-03: four occurrences with minFreq=3 => emitted with Count=4.
func TestExtractNGrams_EmitsAboveMinFreq(t *testing.T) {
	seqs := [][]string{
		{"a", "b", "a", "b"},
		{"a", "b"},
		{"x", "a", "b"},
	}
	got := extractNGrams(seqs, 2, 3)
	if len(got) != 1 || got[0].signature != "a → b" || got[0].count != 4 {
		t.Fatalf("expected a → b count=4, got %+v", got)
	}
}

// TC-D-03a: empty input => empty result.
func TestExtractNGrams_EmptyInputs(t *testing.T) {
	if got := extractNGrams(nil, 2, 1); len(got) != 0 {
		t.Fatalf("nil input expected empty, got %+v", got)
	}
	if got := extractNGrams([][]string{}, 2, 1); len(got) != 0 {
		t.Fatalf("empty slice expected empty, got %+v", got)
	}
	if got := extractNGrams([][]string{{}, {}}, 2, 1); len(got) != 0 {
		t.Fatalf("empty sequences expected empty, got %+v", got)
	}
}

func TestExtractNGrams_SizeLessThanOneReturnsEmpty(t *testing.T) {
	seqs := [][]string{{"a", "b", "c"}}
	if got := extractNGrams(seqs, 0, 1); len(got) != 0 {
		t.Fatalf("size=0 expected empty, got %+v", got)
	}
	if got := extractNGrams(seqs, -1, 1); len(got) != 0 {
		t.Fatalf("size=-1 expected empty, got %+v", got)
	}
}

func TestExtractNGrams_MinFreqLessThanOneTreatedAsOne(t *testing.T) {
	seqs := [][]string{{"a", "b"}}
	got := extractNGrams(seqs, 2, 0)
	if len(got) != 1 || got[0].count != 1 {
		t.Fatalf("minFreq=0 should behave as 1, got %+v", got)
	}
}

func TestExtractNGrams_ShorterSequencesContributeNothing(t *testing.T) {
	seqs := [][]string{
		{"a"},
		{"a", "b"},
		{"a", "b"},
	}
	got := extractNGrams(seqs, 2, 2)
	if len(got) != 1 || got[0].signature != "a → b" || got[0].count != 2 {
		t.Fatalf("expected a → b count=2, got %+v", got)
	}
}

func TestExtractNGrams_OutputSortedBySignature(t *testing.T) {
	seqs := [][]string{
		{"c", "d"},
		{"a", "b"},
		{"e", "f"},
	}
	got := extractNGrams(seqs, 2, 1)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %+v", got)
	}
	sorted := sort.SliceIsSorted(got, func(i, j int) bool { return got[i].signature < got[j].signature })
	if !sorted {
		t.Fatalf("output not sorted: %+v", got)
	}
}

// TC-D-03b: same input twice => byte-identical Marshal output of derived Candidates.
func TestExtractNGrams_DeterministicMarshal(t *testing.T) {
	seqs := [][]string{
		{"a", "b", "c"},
		{"a", "b"},
		{"c", "d"},
		{"c", "d"},
	}
	first := marshalAll(t, seqs)
	second := marshalAll(t, seqs)
	if !bytes.Equal(first, second) {
		t.Fatalf("non-deterministic marshal:\n%s\nvs\n%s", first, second)
	}
}

func marshalAll(t *testing.T, seqs [][]string) []byte {
	t.Helper()
	counted := extractNGrams(seqs, 2, 1)
	ts := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	max := 0
	for _, c := range counted {
		if c.count > max {
			max = c.count
		}
	}
	for _, c := range counted {
		cand := &patternCandidate{
			Kind:        kindToolSequence,
			Signature:   c.signature,
			Frequency:   c.count,
			Contexts:    []string{},
			Score:       scorePattern(c.count, max),
			FirstSeenAt: ts,
			LastSeenAt:  ts,
		}
		b, err := marshalCandidate(cand)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf.Write(b)
	}
	return buf.Bytes()
}

func TestScore(t *testing.T) {
	if got := scorePattern(0, 0); got != 0.0 {
		t.Fatalf("scorePattern(0,0)=%v want 0.0", got)
	}
	if got := scorePattern(5, 10); got != 0.5 {
		t.Fatalf("scorePattern(5,10)=%v want 0.5", got)
	}
	if got := scorePattern(10, 10); got != 1.0 {
		t.Fatalf("scorePattern(10,10)=%v want 1.0", got)
	}
	// Clamp above 1.0 defensively if count > max.
	if got := scorePattern(15, 10); got != 1.0 {
		t.Fatalf("scorePattern(15,10)=%v want clamp 1.0", got)
	}
	// Negative inputs clamp to 0.
	if got := scorePattern(-1, 10); got != 0.0 {
		t.Fatalf("scorePattern(-1,10)=%v want 0.0", got)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return ts
}

func validCandidate(t *testing.T) *patternCandidate {
	t.Helper()
	return &patternCandidate{
		Kind:        kindToolSequence,
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
	c.Kind = patternKind("bogus")
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
	b, err := marshalCandidate(c)
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
	b1, err1 := marshalCandidate(c1)
	b2, err2 := marshalCandidate(c2)
	if err1 != nil || err2 != nil {
		t.Fatalf("marshal errors: %v %v", err1, err2)
	}
	if string(b1) != string(b2) {
		t.Fatalf("marshal not deterministic:\n%s\nvs\n%s", b1, b2)
	}
}

// TC-D-14: extra unknown field => unmarshalCandidateStrict errors.
func TestUnmarshalStrict_RejectsUnknownField(t *testing.T) {
	line := []byte(`{"kind":"tool-sequence","signature":"a → b","frequency":1,"contexts":[],"score":0.1,"first_seen_at":"2026-05-01T00:00:00Z","last_seen_at":"2026-05-02T00:00:00Z","extra":"hi"}`)
	_, err := unmarshalCandidateStrict(line)
	if err == nil || !strings.Contains(err.Error(), "extra") {
		t.Fatalf("expected unknown field error citing 'extra', got %v", err)
	}
}

// TC-D-14: missing kind => patternCandidate parses, Validate fails.
func TestUnmarshalStrict_MissingKindParsesButFailsValidation(t *testing.T) {
	line := []byte(`{"signature":"a → b","frequency":1,"contexts":[],"score":0.1,"first_seen_at":"2026-05-01T00:00:00Z","last_seen_at":"2026-05-02T00:00:00Z"}`)
	c, err := unmarshalCandidateStrict(line)
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
	b, err := marshalCandidate(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := unmarshalCandidateStrict(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Kind != c.Kind || got.Signature != c.Signature || got.Frequency != c.Frequency || got.Score != c.Score {
		t.Fatalf("roundtrip mismatch: got %+v want %+v", got, c)
	}
}
