package audit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
)

func TestAppend_writesOneJSONLineAndCreatesDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "learning")
	e := Entry{
		DecisionID:         42,
		Action:             "applied",
		SourcePath:         "skills/foo/SKILL.md",
		DeprecatedPath:     "skills/_deprecated/foo-20260514T120000Z.md",
		MergedInto:         "skills/foo-canonical/SKILL.md",
		CandidateSignature: "merge:foo+bar",
		Rationale:          "near-duplicate body",
	}
	if appendErr := Append(dir, e); appendErr != nil {
		t.Fatalf("Append: %v", appendErr)
	}
	raw, readErr := os.ReadFile(filepath.Join(dir, FileName))
	if readErr != nil {
		t.Fatalf("read audit file: %v", readErr)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Errorf("audit line should end in newline; got %q", raw)
	}
	var got Entry
	if unmarshalErr := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &got); unmarshalErr != nil {
		t.Fatalf("unmarshal: %v", unmarshalErr)
	}
	if got.DecisionID != 42 || got.Action != "applied" || got.MergedInto == "" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.Timestamp == "" {
		t.Errorf("Append must default Timestamp")
	}
}

func TestAppend_appendsRatherThanTruncates(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "learning")
	for _, id := range []int64{1, 2, 3} {
		if appendErr := Append(dir, Entry{DecisionID: id, Action: "applied", SourcePath: "x"}); appendErr != nil {
			t.Fatalf("Append: %v", appendErr)
		}
	}
	raw, readErr := os.ReadFile(filepath.Join(dir, FileName))
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}
	if got := strings.Count(string(raw), "\n"); got != 3 {
		t.Errorf("want 3 lines, got %d:\n%s", got, raw)
	}
}

func TestRead_missingFileReturnsNil(t *testing.T) {
	t.Parallel()
	entries, readErr := Read(t.TempDir())
	if readErr != nil {
		t.Fatalf("Read on missing file should not error: %v", readErr)
	}
	if entries != nil {
		t.Errorf("want nil, got %+v", entries)
	}
}

func TestRead_skipsMalformedLines(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "learning")
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	mixed := `{"decision_id":1,"action":"applied","source_path":"a"}
this is not json
{"decision_id":2,"action":"rolled-back","source_path":"b"}
`
	if writeErr := os.WriteFile(filepath.Join(dir, FileName), []byte(mixed), 0o600); writeErr != nil {
		t.Fatalf("seed: %v", writeErr)
	}
	entries, readErr := Read(dir)
	if readErr != nil {
		t.Fatalf("Read: %v", readErr)
	}
	if len(entries) != 2 {
		t.Errorf("want 2 valid entries, got %d", len(entries))
	}
}

func TestFindByDecisionID_returnsLatestEntry(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "learning")
	for _, e := range []Entry{
		{DecisionID: 7, Action: "applied", SourcePath: "old"},
		{DecisionID: 8, Action: "applied", SourcePath: "x"},
		{DecisionID: 7, Action: "rolled-back", SourcePath: "new"}, // latest for 7
	} {
		if appendErr := Append(dir, e); appendErr != nil {
			t.Fatalf("Append: %v", appendErr)
		}
	}
	got, findErr := FindByDecisionID(dir, 7)
	if findErr != nil {
		t.Fatalf("FindByDecisionID: %v", findErr)
	}
	if got == nil {
		t.Fatalf("want entry, got nil")
	}
	if got.Action != "rolled-back" || got.SourcePath != "new" {
		t.Errorf("want latest (rolled-back/new), got %+v", got)
	}
}

func TestFindByDecisionID_missingReturnsNil(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "learning")
	if appendErr := Append(dir, Entry{DecisionID: 1, Action: "applied", SourcePath: "x"}); appendErr != nil {
		t.Fatalf("Append: %v", appendErr)
	}
	got, findErr := FindByDecisionID(dir, 99)
	if findErr != nil {
		t.Fatalf("FindByDecisionID: %v", findErr)
	}
	if got != nil {
		t.Errorf("want nil for unknown id, got %+v", got)
	}
}

func TestAppend_failsOnUnwritableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses 0o500 perm; skip")
	}
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "learning")
	if mkErr := os.MkdirAll(dir, 0o500); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	err := Append(dir, Entry{DecisionID: 1, Action: "applied", SourcePath: "x"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var rt *learnerr.RuntimeError
	if !errors.As(err, &rt) {
		t.Errorf("want *learnerr.RuntimeError, got %T", err)
	}
}
