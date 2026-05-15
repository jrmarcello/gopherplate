package transcript

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/sanitize"
)

// newTestSanitizer builds a Sanitizer from the project's default secret
// patterns. Fails the test on compile error.
func newTestSanitizer(t *testing.T) *sanitize.Sanitizer {
	t.Helper()
	s, err := sanitize.BuiltinPatterns()
	if err != nil {
		t.Fatalf("BuiltinPatterns: %v", err)
	}
	return sanitize.New(s)
}

// captureSlog installs a slog handler writing to a buffer for the duration
// of the test (restored via t.Cleanup). Returns the buffer so the caller
// can assert on warn lines.
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestParseFile_HappyPath_ValidJSONL(t *testing.T) {
	san := newTestSanitizer(t)
	recs, err := ParseFile(context.Background(), "testdata/transcripts/valid.jsonl", san)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	// valid.jsonl has 5 lines: user(text), assistant(Bash), queue-op (ignored),
	// assistant(Read), assistant(Edit). Expect 4 records (user-prompt + 3 tool-use).
	if len(recs) != 4 {
		t.Fatalf("expected 4 records, got %d: %+v", len(recs), recs)
	}

	// Source is the filename stem.
	for i, r := range recs {
		if r.Source != "valid" {
			t.Errorf("rec[%d] Source = %q, want %q", i, r.Source, "valid")
		}
		if r.SeenAt.IsZero() {
			t.Errorf("rec[%d] SeenAt is zero", i)
		}
	}

	// First record is user prompt; rest are tool uses.
	if recs[0].Kind != "transcript-user-prompt" {
		t.Errorf("rec[0].Kind = %q, want transcript-user-prompt", recs[0].Kind)
	}
	wantTools := []string{"Bash", "Read", "Edit"}
	for i, want := range wantTools {
		r := recs[i+1]
		if r.Kind != "transcript-tool-use" {
			t.Errorf("rec[%d].Kind = %q, want transcript-tool-use", i+1, r.Kind)
		}
		if len(r.Tokens) != 1 || r.Tokens[0] != want {
			t.Errorf("rec[%d].Tokens = %v, want [%s]", i+1, r.Tokens, want)
		}
		if !strings.Contains(r.Body, want) {
			t.Errorf("rec[%d].Body = %q, want it to contain %q", i+1, r.Body, want)
		}
	}
}

// TC-UC-09: malformed line logs a warn, other lines still parsed.
func TestParseFile_MalformedLine_WarnsAndContinues(t *testing.T) {
	buf := captureSlog(t)
	san := newTestSanitizer(t)
	recs, err := ParseFile(context.Background(), "testdata/transcripts/malformed.jsonl", san)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	// line1 = user-prompt, line2 = bad, line3 = assistant tool_use Grep.
	// Expect 2 records.
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d: %+v", len(recs), recs)
	}
	if recs[0].Kind != "transcript-user-prompt" {
		t.Errorf("rec[0].Kind = %q", recs[0].Kind)
	}
	if recs[1].Kind != "transcript-tool-use" || recs[1].Tokens[0] != "Grep" {
		t.Errorf("rec[1] = %+v", recs[1])
	}
	out := buf.String()
	if !strings.Contains(out, "transcript skip") {
		t.Errorf("expected slog warn 'transcript skip' in output, got: %s", out)
	}
}

// TC-UC-10: permission-denied file → RuntimeError.
func TestParseFile_PermissionDenied_ReturnsRuntimeError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0 unreliable on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root, chmod 0 won't block reads")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "denied.jsonl")
	if writeErr := os.WriteFile(path, []byte(`{"type":"user"}`+"\n"), 0o644); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	if chmodErr := os.Chmod(path, 0); chmodErr != nil {
		t.Fatalf("chmod 0: %v", chmodErr)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	san := newTestSanitizer(t)
	_, err := ParseFile(context.Background(), path, san)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var re *learnerr.RuntimeError
	if !errors.As(err, &re) {
		t.Fatalf("expected *learnerr.RuntimeError, got %T: %v", err, err)
	}
}

// TC-UC-11a: ParseDir on nonexistent dir → RuntimeError.
func TestParseDir_MissingDir_ReturnsRuntimeError(t *testing.T) {
	san := newTestSanitizer(t)
	_, err := ParseDir(context.Background(), "testdata/no-such-dir", san)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var re *learnerr.RuntimeError
	if !errors.As(err, &re) {
		t.Fatalf("expected *learnerr.RuntimeError, got %T: %v", err, err)
	}
}

// TC-UC-11b: ParseDir on a dir with valid+malformed jsonl → returns valid
// records and warns about malformed lines.
func TestParseDir_MixedFiles_SkipsAndWarns(t *testing.T) {
	dir := t.TempDir()
	// Copy valid + malformed under UUID-like names so the pattern matches.
	for _, src := range []string{"valid.jsonl", "malformed.jsonl"} {
		data, readErr := os.ReadFile(filepath.Join("testdata/transcripts", src))
		if readErr != nil {
			t.Fatalf("read %s: %v", src, readErr)
		}
		// 32 hex chars + dashes-ish; just use 32 chars to satisfy the [a-f0-9-]{32,36}\.jsonl pattern.
		stem := strings.TrimSuffix(src, ".jsonl")
		// Build a 36-char UUID-ish name embedding the stem hint isn't needed; use stable hex.
		var name string
		if stem == "valid" {
			name = "11111111-1111-1111-1111-111111111111.jsonl"
		} else {
			name = "22222222-2222-2222-2222-222222222222.jsonl"
		}
		if writeErr := os.WriteFile(filepath.Join(dir, name), data, 0o644); writeErr != nil {
			t.Fatalf("write %s: %v", name, writeErr)
		}
	}
	// Add a non-matching file (should be ignored).
	if writeErr := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644); writeErr != nil {
		t.Fatalf("write notes: %v", writeErr)
	}

	buf := captureSlog(t)
	san := newTestSanitizer(t)
	recs, err := ParseDir(context.Background(), dir, san)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	// valid contributes 4, malformed contributes 2 → 6 total.
	if len(recs) != 6 {
		t.Fatalf("expected 6 records, got %d", len(recs))
	}
	if !strings.Contains(buf.String(), "transcript skip") {
		t.Errorf("expected at least one 'transcript skip' warn for malformed line, got: %s", buf.String())
	}
}

// REQ-4 / TC-D-07a: secrets in tool_use input.command are sanitized in Body.
func TestParseFile_SecretsRedactedInBody(t *testing.T) {
	san := newTestSanitizer(t)
	recs, err := ParseFile(context.Background(), "testdata/transcripts/with-secrets.jsonl", san)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	body := recs[0].Body
	if strings.Contains(body, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("Body must not contain raw AWS key, got: %q", body)
	}
	if !strings.Contains(body, "<REDACTED:aws_key>") {
		t.Errorf("Body must contain <REDACTED:aws_key> token, got: %q", body)
	}
	// Tokens (tool name) should still be Bash.
	if recs[0].Tokens[0] != "Bash" {
		t.Errorf("Tokens[0] = %q, want Bash", recs[0].Tokens[0])
	}
}

// Sanity: context cancellation aborts the line loop.
func TestParseFile_ContextCancel(t *testing.T) {
	san := newTestSanitizer(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ParseFile(ctx, "testdata/transcripts/valid.jsonl", san)
	if err == nil {
		t.Fatalf("expected context error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// ensure io.EOF on an empty file is a clean exit.
func TestParseFile_EmptyFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if writeErr := os.WriteFile(path, nil, 0o644); writeErr != nil {
		t.Fatalf("write empty: %v", writeErr)
	}
	san := newTestSanitizer(t)
	recs, err := ParseFile(context.Background(), path, san)
	if err != nil {
		t.Fatalf("ParseFile empty: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected 0 recs, got %d", len(recs))
	}
	// Avoid an unused-import warning on io if all calls drop.
	_ = io.EOF
}
