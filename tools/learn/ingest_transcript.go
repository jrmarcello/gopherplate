// Claude Code transcript parser. Reads JSONL files into a stream of
// [transcriptRecord]s suitable for the learning-loop store.
//
// Transcripts live at ~/.claude/projects/<dir>/<uuid>.jsonl. Each line is a
// self-contained JSON object describing one event: a user message, an
// assistant message (possibly containing tool_use items), or some opaque
// system event (queue-operation, etc.). Unknown event types are skipped
// silently.
//
// Two record kinds are emitted:
//
//   - "transcript-user-prompt" — user text messages.
//   - "transcript-tool-use"    — assistant tool invocations; Tokens carries
//     the canonical tool name (Bash, Read, Edit, ...).
//
// Errors per line (invalid JSON, missing fields, truncated) emit a
// structured slog warn and CONTINUE to the next line. File-level failures
// (permission denied, ENOENT, "is a directory") surface as
// *runtimeError so the caller can map them to exit code 2.
//
// All Body and Tokens content passes through the [sanitizer] before being
// returned — REQ-4 mandates that no caller observes unsanitized strings from
// this file.

package learn

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// transcriptRecord is the canonical shared output of the transcript parser.
type transcriptRecord struct {
	// Source is the session-id (filename stem of the JSONL file).
	Source string
	// Kind is one of "transcript-tool-use" or "transcript-user-prompt".
	Kind string
	// Tokens carries semantic features extracted from the line. For tool-use
	// records this is a single-element slice with the tool name. Always
	// emitted in chronological order across the file.
	Tokens []string
	// SeenAt is the line timestamp parsed as RFC3339.
	SeenAt time.Time
	// Body is a sanitized excerpt suitable for similarity hashing / display.
	Body string
}

// uuidFilePattern matches filenames Claude Code uses to store transcripts:
// a hex-and-dash session id between 32 and 36 chars long, followed by .jsonl.
// We are tolerant of variations seen in the wild (dashes optional).
var uuidFilePattern = regexp.MustCompile(`^[a-f0-9-]{32,36}\.jsonl$`)

// rawLine is the minimal JSON shape we decode per line. Unknown fields are
// ignored by encoding/json by default.
type rawLine struct {
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	Message   *rawMessage `json:"message,omitempty"`
}

type rawMessage struct {
	Role    string           `json:"role"`
	Content []rawContentItem `json:"content"`
}

type rawContentItem struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// parseTranscriptFile reads one JSONL file and returns the records it
// contains.
//
// File-level failures map to *runtimeError:
//
//   - permission denied
//   - ENOENT (no such file)
//   - "is a directory"
//
// Per-line failures (invalid JSON, missing fields) log a structured slog
// warn and continue. EOF is a clean exit.
//
// The sanitizer is applied to Body and to any Tokens value before return.
func parseTranscriptFile(ctx context.Context, path string, san *sanitizer) ([]transcriptRecord, error) {
	f, openErr := os.Open(path) //nolint:gosec // caller-controlled transcript path
	if openErr != nil {
		return nil, &runtimeError{
			Msg: fmt.Sprintf("transcript: open %s", path),
			Err: openErr,
		}
	}
	defer func() { _ = f.Close() }()

	// Reject directories explicitly — os.Open succeeds on a dir, but the
	// subsequent read returns a less actionable error.
	info, statErr := f.Stat()
	if statErr != nil {
		return nil, &runtimeError{
			Msg: fmt.Sprintf("transcript: stat %s", path),
			Err: statErr,
		}
	}
	if info.IsDir() {
		return nil, &runtimeError{
			Msg: fmt.Sprintf("transcript: %s is a directory", path),
		}
	}

	source := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	scanner := bufio.NewScanner(f)
	// Allow long lines — assistant tool_use inputs can be sizeable, but cap
	// to avoid pathological inputs causing OOM in reindex passes.
	const maxLine = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 64*1024), maxLine)

	var out []transcriptRecord
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		line := scanner.Bytes()
		// bytes.TrimSpace avoids the []byte→string allocation that
		// strings.TrimSpace(string(line)) incurs in this hot path.
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		// Per-line value cap: skip lines exceeding maxRecordBytes (256KB)
		// after scanner already gave us bytes within maxLine. This is a
		// defense-in-depth value cap independent of the scanner buffer:
		// pathological inputs warn+skip instead of unmarshal-and-balloon-RSS.
		const maxRecordBytes = 256 * 1024
		if len(line) > maxRecordBytes {
			slog.WarnContext(ctx, "transcript skip",
				"source", source,
				"line", lineNo,
				"reason", fmt.Sprintf("line exceeds %d bytes (got %d) — value cap", maxRecordBytes, len(line)),
			)
			continue
		}
		recs, parseErr := parseTranscriptLine(line, source, san)
		if parseErr != nil {
			slog.WarnContext(ctx, "transcript skip",
				"source", source,
				"line", lineNo,
				"reason", parseErr.Error(),
			)
			continue
		}
		out = append(out, recs...)
	}
	if scanErr := scanner.Err(); scanErr != nil && !errors.Is(scanErr, io.EOF) {
		return nil, &runtimeError{
			Msg: fmt.Sprintf("transcript: scan %s", path),
			Err: scanErr,
		}
	}
	return out, nil
}

// parseTranscriptDir walks dir (non-recursive) collecting records from every
// file whose name matches uuidFilePattern. Per-file errors log a warn and
// skip; only a missing/unreadable directory itself produces a *runtimeError.
func parseTranscriptDir(ctx context.Context, dir string, san *sanitizer) ([]transcriptRecord, error) {
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return nil, &runtimeError{
			Msg: fmt.Sprintf("transcript: read dir %s", dir),
			Err: readErr,
		}
	}
	var out []transcriptRecord
	for _, e := range entries {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !uuidFilePattern.MatchString(name) {
			continue
		}
		full := filepath.Join(dir, name)
		recs, parseErr := parseTranscriptFile(ctx, full, san)
		if parseErr != nil {
			slog.WarnContext(ctx, "transcript skip",
				"source", strings.TrimSuffix(name, filepath.Ext(name)),
				"reason", parseErr.Error(),
			)
			continue
		}
		out = append(out, recs...)
	}
	return out, nil
}

// parseTranscriptLine decodes one JSONL line into zero or more
// TranscriptRecords. A line may expand to multiple records when an assistant
// message contains several tool_use content items.
//
// Returning an error here means the line is malformed and the caller will
// log + skip. Unknown-but-well-formed event types return (nil, nil).
func parseTranscriptLine(line []byte, source string, san *sanitizer) ([]transcriptRecord, error) {
	var raw rawLine
	if unmarshalErr := json.Unmarshal(line, &raw); unmarshalErr != nil {
		return nil, fmt.Errorf("invalid json: %w", unmarshalErr)
	}

	seenAt, tsErr := parseTranscriptTimestamp(raw.Timestamp)
	if tsErr != nil {
		// Non-fatal at line level — but if timestamp is missing AND we'd emit
		// a record, return a warn so the caller skips this line. The spec is
		// explicit: "missing fields" → warn + skip.
		return nil, tsErr
	}

	switch raw.Type {
	case "user":
		return extractUserRecord(raw, source, seenAt, san), nil
	case "assistant":
		return extractAssistantRecords(raw, source, seenAt, san), nil
	default:
		// Unknown / ignorable event (queue-operation, system, etc.).
		return nil, nil
	}
}

// extractUserRecord pulls the user's text content into a single record.
// Returns nil if no text content was found (the line is well-formed but
// not interesting).
func extractUserRecord(raw rawLine, source string, ts time.Time, san *sanitizer) []transcriptRecord {
	if raw.Message == nil {
		return nil
	}
	var b strings.Builder
	for _, c := range raw.Message.Content {
		if c.Type == "text" && c.Text != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(c.Text)
		}
	}
	if b.Len() == 0 {
		return nil
	}
	body := san.sanitize(b.String())
	return []transcriptRecord{{
		Source: source,
		Kind:   "transcript-user-prompt",
		SeenAt: ts,
		Body:   body,
	}}
}

// extractAssistantRecords pulls each tool_use item out as its own
// transcriptRecord. Tool name becomes the single Token; Body is
// "ToolName: <short args>".
func extractAssistantRecords(raw rawLine, source string, ts time.Time, san *sanitizer) []transcriptRecord {
	if raw.Message == nil {
		return nil
	}
	var out []transcriptRecord
	for _, c := range raw.Message.Content {
		if c.Type != "tool_use" || c.Name == "" {
			continue
		}
		summary := summarizeToolUse(c.Name, c.Input)
		body := san.sanitize(summary)
		token := san.sanitize(c.Name)
		out = append(out, transcriptRecord{
			Source: source,
			Kind:   "transcript-tool-use",
			Tokens: []string{token},
			SeenAt: ts,
			Body:   body,
		})
	}
	return out
}

// summarizeToolUse builds a short single-line excerpt of a tool_use payload
// for the Body field. We avoid dumping the full input — just the tool name
// and a truncated JSON-ish view of the input.
func summarizeToolUse(name string, input json.RawMessage) string {
	if len(input) == 0 {
		return name
	}
	const maxArgs = 256
	args := string(input)
	if len(args) > maxArgs {
		args = args[:maxArgs] + "…"
	}
	return name + ": " + args
}

// parseTranscriptTimestamp accepts RFC3339 (with or without nanoseconds). An
// empty timestamp returns an error so the caller skips and warns — REQ-3 says
// missing fields are treated as malformed.
func parseTranscriptTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("missing timestamp")
	}
	if t, parseErr := time.Parse(time.RFC3339Nano, s); parseErr == nil {
		return t, nil
	}
	t, parseErr := time.Parse(time.RFC3339, s)
	if parseErr != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp %q: %w", s, parseErr)
	}
	return t, nil
}
