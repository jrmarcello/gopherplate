package learn

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func specFixture(name string) string {
	return filepath.Join("testdata", "specs", name)
}

func TestParse_MinimalDone(t *testing.T) {
	t.Parallel()

	san := newTestSanitizer(t)
	records, parseErr := parseSpec(context.Background(), specFixture("minimal-done.md"), san)
	if parseErr != nil {
		t.Fatalf("parseSpec: %v", parseErr)
	}

	var kindCounts = map[string]int{}
	var sources = map[string]bool{}
	for _, r := range records {
		kindCounts[r.Kind]++
		sources[r.Source] = true
	}

	if kindCounts["spec-status-transition"] != 1 {
		t.Errorf("status-transition: want 1, got %d", kindCounts["spec-status-transition"])
	}
	if kindCounts["spec-task"] != 1 {
		t.Errorf("spec-task (completed only): want 1, got %d", kindCounts["spec-task"])
	}
	// Two log entries: TASK-1 header + Batch 2 header.
	if kindCounts["spec-tdd-step"] != 2 {
		t.Errorf("spec-tdd-step: want 2, got %d", kindCounts["spec-tdd-step"])
	}
	for src := range sources {
		if !strings.HasSuffix(src, "minimal-done.md") {
			t.Errorf("Source %q should reference the spec file", src)
		}
	}

	// Find the spec-task record and verify Tokens carry TASK-id + files info.
	var taskRec *specRecord
	for i := range records {
		if records[i].Kind == "spec-task" {
			taskRec = &records[i]
			break
		}
	}
	if taskRec == nil {
		t.Fatal("expected one spec-task record")
	}
	joined := strings.Join(taskRec.Tokens, ",")
	if !strings.Contains(joined, "TASK-1") {
		t.Errorf("spec-task tokens should include TASK-1, got %v", taskRec.Tokens)
	}
	if !strings.Contains(joined, "foo") {
		t.Errorf("spec-task tokens should reference files containing 'foo', got %v", taskRec.Tokens)
	}

	// TDD step records: first one (TASK-1) should have ["RED","GREEN","REFACTOR"].
	var firstTDD *specRecord
	for i := range records {
		if records[i].Kind == "spec-tdd-step" {
			firstTDD = &records[i]
			break
		}
	}
	if firstTDD == nil {
		t.Fatal("expected at least one spec-tdd-step record")
	}
	if len(firstTDD.Tokens) != 3 ||
		firstTDD.Tokens[0] != "RED" ||
		firstTDD.Tokens[1] != "GREEN" ||
		firstTDD.Tokens[2] != "REFACTOR" {
		t.Errorf("first spec-tdd-step Tokens: want [RED GREEN REFACTOR], got %v", firstTDD.Tokens)
	}
}

func TestParse_SeenAtPopulated_TC_UC_08(t *testing.T) {
	t.Parallel()

	san := newTestSanitizer(t)
	records, parseErr := parseSpec(context.Background(), specFixture("minimal-done.md"), san)
	if parseErr != nil {
		t.Fatalf("parseSpec: %v", parseErr)
	}

	// Filter the TDD steps and assert SeenAt is non-zero and parseable from
	// the "2026-05-14 18:42" header. TC-UC-08 caller-side filter contract.
	want, _ := time.Parse("2006-01-02 15:04", "2026-05-14 18:42")
	found := false
	for _, r := range records {
		if r.Kind != "spec-tdd-step" {
			continue
		}
		if r.SeenAt.IsZero() {
			t.Errorf("spec-tdd-step %q: SeenAt must not be zero", r.Source)
		}
		if r.SeenAt.Equal(want) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected at least one spec-tdd-step with SeenAt = %v", want)
	}
}

func TestParse_DraftSkipsTransitionAndTasks(t *testing.T) {
	t.Parallel()

	san := newTestSanitizer(t)
	records, parseErr := parseSpec(context.Background(), specFixture("draft.md"), san)
	if parseErr != nil {
		t.Fatalf("parseSpec: %v", parseErr)
	}

	for _, r := range records {
		if r.Kind == "spec-status-transition" {
			t.Errorf("DRAFT spec must NOT emit spec-status-transition, got %+v", r)
		}
		if r.Kind == "spec-task" {
			t.Errorf("DRAFT spec has no [x] tasks; got spec-task %+v", r)
		}
	}
}

func TestParse_EmptyFile(t *testing.T) {
	t.Parallel()

	san := newTestSanitizer(t)
	records, parseErr := parseSpec(context.Background(), specFixture("empty.md"), san)
	if parseErr != nil {
		t.Fatalf("parseSpec(empty): %v", parseErr)
	}
	if len(records) != 0 {
		t.Errorf("empty file should yield zero records, got %d", len(records))
	}
}

func TestParse_MissingStatusHeader(t *testing.T) {
	t.Parallel()

	// Build a fake spec inline by writing to a temp file: no `## Status:` line,
	// but a [x] task and a TDD log entry.
	dir := t.TempDir()
	path := filepath.Join(dir, "no-status.md")
	body := `# Spec

## Tasks

- [x] TASK-9 do thing
  - files: ` + "`a/b.go`" + `

## Execution Log

### TASK-9 (2026-05-14 19:00)

TDD: RED(1) → GREEN(1) → REFACTOR(clean).
`
	if writeErr := writeSpecFile(path, body); writeErr != nil {
		t.Fatalf("write fixture: %v", writeErr)
	}

	san := newTestSanitizer(t)
	records, parseErr := parseSpec(context.Background(), path, san)
	if parseErr != nil {
		t.Fatalf("parseSpec: %v", parseErr)
	}
	for _, r := range records {
		if r.Kind == "spec-status-transition" {
			t.Fatalf("missing status header must skip transition record, got %+v", r)
		}
	}
	// Still emits task + tdd-step.
	var sawTask, sawTDD bool
	for _, r := range records {
		if r.Kind == "spec-task" {
			sawTask = true
		}
		if r.Kind == "spec-tdd-step" {
			sawTDD = true
		}
	}
	if !sawTask || !sawTDD {
		t.Errorf("expected spec-task and spec-tdd-step despite missing status, got %v", records)
	}
}

func TestParse_SanitizesAWSKey_TC_UC_08(t *testing.T) {
	t.Parallel()

	san := newTestSanitizer(t)
	records, parseErr := parseSpec(context.Background(), specFixture("with-aws-key.md"), san)
	if parseErr != nil {
		t.Fatalf("parseSpec: %v", parseErr)
	}
	for _, r := range records {
		if strings.Contains(r.Body, "AKIAABCDEFGHIJKLMNOP") {
			t.Errorf("Body must be sanitized; raw AKIA leaked in record %s/%s: %q", r.Kind, r.Source, r.Body)
		}
		for _, tok := range r.Tokens {
			if strings.Contains(tok, "AKIAABCDEFGHIJKLMNOP") {
				t.Errorf("Tokens must be sanitized; raw AKIA leaked in %v", r.Tokens)
			}
		}
	}

	// Locate the TASK-1 log entry and confirm the redaction marker is present.
	found := false
	for _, r := range records {
		if r.Kind != "spec-tdd-step" {
			continue
		}
		if strings.Contains(r.Body, "<REDACTED:aws_key>") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected <REDACTED:aws_key> marker in some spec-tdd-step Body")
	}

	// TASK-2 has no TDD line — record must still be emitted with empty Tokens.
	var task2 *specRecord
	for i := range records {
		if records[i].Kind == "spec-tdd-step" && strings.Contains(records[i].Body, "TASK-2") {
			task2 = &records[i]
			break
		}
	}
	if task2 == nil {
		t.Fatal("TASK-2 log entry should still emit a spec-tdd-step record")
	}
	if len(task2.Tokens) != 0 {
		t.Errorf("TASK-2 has no TDD line; Tokens must be empty, got %v", task2.Tokens)
	}
}

func TestParse_FileNotFound_ReturnsRuntimeError(t *testing.T) {
	t.Parallel()

	san := newTestSanitizer(t)
	_, parseErr := parseSpec(context.Background(), specFixture("does-not-exist.md"), san)
	if parseErr == nil {
		t.Fatal("parseSpec: expected error for missing file")
	}
	var re *runtimeError
	if !errors.As(parseErr, &re) {
		t.Fatalf("expected *runtimeError, got %T: %v", parseErr, parseErr)
	}
	if !errors.Is(parseErr, fs.ErrNotExist) {
		t.Errorf("error should wrap fs.ErrNotExist; got %v", parseErr)
	}
}

// writeSpecFile is a tiny helper used by tests that synthesize a fixture on
// disk instead of placing it under testdata/specs/.
func writeSpecFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o600)
}
