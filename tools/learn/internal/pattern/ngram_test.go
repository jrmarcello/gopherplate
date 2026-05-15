package pattern

import (
	"bytes"
	"sort"
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
	got := ExtractNGrams(seqs, 2, 3)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(got), got)
	}
	if got[0].Signature != "a → b" || got[0].Count != 3 {
		t.Fatalf("expected a → b count=3, got %+v", got[0])
	}
}

// TC-D-02: minFreq=3, only two occurrences => NOT emitted.
func TestExtractNGrams_OmitsWhenCountBelowMinFreq(t *testing.T) {
	seqs := [][]string{
		{"a", "b", "c"},
		{"a", "b", "d"},
	}
	got := ExtractNGrams(seqs, 2, 3)
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
	got := ExtractNGrams(seqs, 2, 3)
	if len(got) != 1 || got[0].Signature != "a → b" || got[0].Count != 4 {
		t.Fatalf("expected a → b count=4, got %+v", got)
	}
}

// TC-D-03a: empty input => empty result.
func TestExtractNGrams_EmptyInputs(t *testing.T) {
	if got := ExtractNGrams(nil, 2, 1); len(got) != 0 {
		t.Fatalf("nil input expected empty, got %+v", got)
	}
	if got := ExtractNGrams([][]string{}, 2, 1); len(got) != 0 {
		t.Fatalf("empty slice expected empty, got %+v", got)
	}
	if got := ExtractNGrams([][]string{{}, {}}, 2, 1); len(got) != 0 {
		t.Fatalf("empty sequences expected empty, got %+v", got)
	}
}

func TestExtractNGrams_SizeLessThanOneReturnsEmpty(t *testing.T) {
	seqs := [][]string{{"a", "b", "c"}}
	if got := ExtractNGrams(seqs, 0, 1); len(got) != 0 {
		t.Fatalf("size=0 expected empty, got %+v", got)
	}
	if got := ExtractNGrams(seqs, -1, 1); len(got) != 0 {
		t.Fatalf("size=-1 expected empty, got %+v", got)
	}
}

func TestExtractNGrams_MinFreqLessThanOneTreatedAsOne(t *testing.T) {
	seqs := [][]string{{"a", "b"}}
	got := ExtractNGrams(seqs, 2, 0)
	if len(got) != 1 || got[0].Count != 1 {
		t.Fatalf("minFreq=0 should behave as 1, got %+v", got)
	}
}

func TestExtractNGrams_ShorterSequencesContributeNothing(t *testing.T) {
	seqs := [][]string{
		{"a"},
		{"a", "b"},
		{"a", "b"},
	}
	got := ExtractNGrams(seqs, 2, 2)
	if len(got) != 1 || got[0].Signature != "a → b" || got[0].Count != 2 {
		t.Fatalf("expected a → b count=2, got %+v", got)
	}
}

func TestExtractNGrams_OutputSortedBySignature(t *testing.T) {
	seqs := [][]string{
		{"c", "d"},
		{"a", "b"},
		{"e", "f"},
	}
	got := ExtractNGrams(seqs, 2, 1)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %+v", got)
	}
	sorted := sort.SliceIsSorted(got, func(i, j int) bool { return got[i].Signature < got[j].Signature })
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
	counted := ExtractNGrams(seqs, 2, 1)
	ts := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	max := 0
	for _, c := range counted {
		if c.Count > max {
			max = c.Count
		}
	}
	for _, c := range counted {
		cand := &Candidate{
			Kind:        KindToolSequence,
			Signature:   c.Signature,
			Frequency:   c.Count,
			Contexts:    []string{},
			Score:       Score(c.Count, max),
			FirstSeenAt: ts,
			LastSeenAt:  ts,
		}
		b, err := Marshal(cand)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf.Write(b)
	}
	return buf.Bytes()
}

func TestScore(t *testing.T) {
	if got := Score(0, 0); got != 0.0 {
		t.Fatalf("Score(0,0)=%v want 0.0", got)
	}
	if got := Score(5, 10); got != 0.5 {
		t.Fatalf("Score(5,10)=%v want 0.5", got)
	}
	if got := Score(10, 10); got != 1.0 {
		t.Fatalf("Score(10,10)=%v want 1.0", got)
	}
	// Clamp above 1.0 defensively if count > max.
	if got := Score(15, 10); got != 1.0 {
		t.Fatalf("Score(15,10)=%v want clamp 1.0", got)
	}
	// Negative inputs clamp to 0.
	if got := Score(-1, 10); got != 0.0 {
		t.Fatalf("Score(-1,10)=%v want 0.0", got)
	}
}
