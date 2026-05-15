// Package spec parses SDD spec files (`.specs/*.md`) into ingest Records
// suitable for the learning-loop store (REQ-2 of the
// learning-loop-harness spec).
//
// The parser is structural, not a full markdown implementation. It only
// recognizes the anchors used by the project's spec template:
//
//   - `## Status: <STATE>` — extracted as a spec-status-transition Record
//     when the spec has reached IN_PROGRESS or DONE.
//   - `- [x] TASK-N <title>` — extracted as a spec-task Record. Sub-bullets
//     beginning with `files:` are tokenized.
//   - `### TASK-N (timestamp)` and `### Batch ...` headers within an
//     `## Execution Log` section — extracted as spec-tdd-step Records with
//     the "TDD: RED(X) → GREEN(Y) → REFACTOR(...)" sequence tokenized as
//     ["RED", "GREEN", "REFACTOR"].
//
// All Body and Tokens fields are passed through the supplied Sanitizer
// before being returned. The parser never returns unredacted text.
package spec

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/sanitize"
)

// Record is one parseable unit from a single source. Across all four
// ingest packages (spec, git, transcript, memory) the shape is identical so
// downstream extract/reindex commands can iterate uniformly.
type Record struct {
	Source string    // canonical identifier — `.specs/foo.md`, sha, session id, etc.
	Kind   string    // "spec-status-transition" | "spec-task" | "spec-tdd-step"
	Tokens []string  // ordered tokens for pattern.ExtractNGrams
	SeenAt time.Time // timestamp from the source (parse time as fallback)
	Body   string    // raw text after sanitization, used for FTS5 indexing
}

// Kind constants used by callers that match on Record.Kind.
const (
	KindStatusTransition = "spec-status-transition"
	KindTask             = "spec-task"
	KindTDDStep          = "spec-tdd-step"
)

// statusEmittedFor reports whether a status warrants emitting a
// spec-status-transition record. DRAFT/APPROVED specs have not started
// execution yet; only IN_PROGRESS or DONE produce a transition signal.
func statusEmittedFor(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "IN_PROGRESS", "DONE", "FAILED":
		return true
	default:
		return false
	}
}

var (
	statusRe       = regexp.MustCompile(`^##\s+Status:\s+([A-Z_]+)\s*$`)
	taskCompleteRe = regexp.MustCompile(`^-\s+\[x\]\s+(TASK-[A-Z0-9_-]+)\b\s*(.*)$`)
	filesLineRe    = regexp.MustCompile(`^\s+-\s+files:\s+(.*)$`)
	logHeaderRe    = regexp.MustCompile(`^###\s+((?:TASK-[A-Z0-9_-]+|Batch[^()]*))\s*\(([^)]+)\)\s*$`)
	tddRe          = regexp.MustCompile(`TDD:.*\bRED\b.*\bGREEN\b.*\bREFACTOR\b`)
)

// Parse reads a .specs/*.md file and returns Records derived from its
// structural anchors. See the package comment for the full contract.
//
// File-not-found is mapped to *learnerr.RuntimeError wrapping fs.ErrNotExist
// so callers can classify the failure via the project's exit-code contract.
// All other read failures are likewise wrapped as RuntimeError. An empty
// file yields a nil slice and a nil error.
//
// The Sanitizer is mandatory; passing nil panics. Body and every token in
// Tokens are sanitized before being attached to a Record.
func Parse(ctx context.Context, specPath string, san *sanitize.Sanitizer) ([]Record, error) {
	if san == nil {
		panic("spec.Parse: sanitizer must not be nil")
	}

	f, openErr := os.Open(specPath) //nolint:gosec // caller-supplied spec path; package purpose is to read it
	if openErr != nil {
		return nil, &learnerr.RuntimeError{
			Msg: fmt.Sprintf("spec: open %s", specPath),
			Err: openErr,
		}
	}
	defer func() { _ = f.Close() }()

	source := canonicalSource(specPath)
	now := time.Now().UTC()

	var (
		records []Record

		// Status state.
		statusSeen string

		// Task collector.
		inTaskBlock bool
		taskID      string
		taskTitle   string
		taskFiles   []string
		taskBodyB   strings.Builder

		// Execution-log state.
		inExecLog       bool
		curLogHeader    string // e.g. "TASK-1" or "Batch 2 [TASK-2, TASK-3]"
		curLogSeenAt    time.Time
		curLogBodyB     strings.Builder
		curLogHasHeader bool
	)

	flushTask := func() {
		if !inTaskBlock {
			return
		}
		tokens := make([]string, 0, 1+len(taskFiles))
		tokens = append(tokens, taskID)
		tokens = append(tokens, taskFiles...)
		// Sanitize body + each token before exposing.
		body := san.Sanitize(strings.TrimRight(taskBodyB.String(), "\n"))
		sanTokens := make([]string, len(tokens))
		for i, t := range tokens {
			sanTokens[i] = san.Sanitize(t)
		}
		records = append(records, Record{
			Source: source,
			Kind:   KindTask,
			Tokens: sanTokens,
			SeenAt: now,
			Body:   body,
		})
		// Reset.
		inTaskBlock = false
		taskID = ""
		taskTitle = ""
		taskFiles = nil
		taskBodyB.Reset()
	}

	flushLog := func() {
		if !curLogHasHeader {
			return
		}
		raw := curLogBodyB.String()
		tokens := extractTDDTokens(raw)
		body := san.Sanitize(strings.TrimRight(raw, "\n"))
		sanTokens := make([]string, len(tokens))
		for i, t := range tokens {
			sanTokens[i] = san.Sanitize(t)
		}
		records = append(records, Record{
			Source: source,
			Kind:   KindTDDStep,
			Tokens: sanTokens,
			SeenAt: curLogSeenAt,
			Body:   body,
		})
		curLogHeader = ""
		curLogSeenAt = time.Time{}
		curLogBodyB.Reset()
		curLogHasHeader = false
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		// Honor context cancellation between lines so very large specs do
		// not pin the goroutine.
		select {
		case <-ctx.Done():
			return nil, &learnerr.RuntimeError{Msg: "spec: parse canceled", Err: ctx.Err()}
		default:
		}
		line := sc.Text()

		// Section transitions.
		if strings.HasPrefix(line, "## ") {
			// Leaving the current execution log section means flushing the
			// pending log entry.
			if inExecLog {
				flushLog()
			}
			// Leaving the tasks section flushes the pending task.
			if inTaskBlock {
				flushTask()
			}
			inExecLog = strings.HasPrefix(line, "## Execution Log")

			if m := statusRe.FindStringSubmatch(line); m != nil {
				statusSeen = m[1]
			}
			continue
		}

		if inExecLog {
			if m := logHeaderRe.FindStringSubmatch(line); m != nil {
				// Flush the previous entry before starting the new one.
				flushLog()
				curLogHeader = strings.TrimSpace(m[1])
				curLogSeenAt = parseLogTimestamp(m[2])
				curLogHasHeader = true
				curLogBodyB.WriteString(curLogHeader)
				curLogBodyB.WriteString("\n")
				continue
			}
			if curLogHasHeader {
				curLogBodyB.WriteString(line)
				curLogBodyB.WriteString("\n")
			}
			continue
		}

		// Outside the execution log: look for completed tasks.
		if m := taskCompleteRe.FindStringSubmatch(line); m != nil {
			// New task — flush any previous one.
			flushTask()
			inTaskBlock = true
			taskID = m[1]
			taskTitle = strings.TrimSpace(m[2])
			taskBodyB.WriteString(taskID)
			if taskTitle != "" {
				taskBodyB.WriteString(" ")
				taskBodyB.WriteString(taskTitle)
			}
			taskBodyB.WriteString("\n")
			continue
		}

		if inTaskBlock {
			if m := filesLineRe.FindStringSubmatch(line); m != nil {
				taskFiles = append(taskFiles, splitFiles(m[1])...)
				taskBodyB.WriteString(line)
				taskBodyB.WriteString("\n")
				continue
			}
			// A non-indented line that is not a task bullet ends the task.
			// We use a heuristic: a blank line or a new top-level bullet
			// closes the task; sub-bullets (indented) belong to it.
			if line == "" {
				flushTask()
				continue
			}
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "#") {
				flushTask()
				// Fall through so a new `- [x]` line can re-open immediately.
				if mm := taskCompleteRe.FindStringSubmatch(line); mm != nil {
					inTaskBlock = true
					taskID = mm[1]
					taskTitle = strings.TrimSpace(mm[2])
					taskBodyB.WriteString(taskID)
					if taskTitle != "" {
						taskBodyB.WriteString(" ")
						taskBodyB.WriteString(taskTitle)
					}
					taskBodyB.WriteString("\n")
				}
				continue
			}
			// Indented continuation.
			taskBodyB.WriteString(line)
			taskBodyB.WriteString("\n")
		}
	}
	if scanErr := sc.Err(); scanErr != nil {
		return nil, &learnerr.RuntimeError{
			Msg: fmt.Sprintf("spec: scan %s", specPath),
			Err: scanErr,
		}
	}

	// Final flushes.
	if inTaskBlock {
		flushTask()
	}
	if inExecLog {
		flushLog()
	}

	// Status-transition record (emitted last so it sits at a stable position
	// after the body records; callers that care about ordering can sort).
	if statusEmittedFor(statusSeen) {
		body := san.Sanitize("Status: " + statusSeen)
		records = append([]Record{{
			Source: source,
			Kind:   KindStatusTransition,
			Tokens: []string{san.Sanitize(strings.ToUpper(statusSeen))},
			SeenAt: now,
			Body:   body,
		}}, records...)
	}

	return records, nil
}

// canonicalSource returns the path normalised for use as the Record.Source.
// We strip the working-directory prefix so test fixtures and production
// specs share the same `.specs/...` shape when possible, but always fall
// back to the original path so the record stays traceable.
func canonicalSource(p string) string {
	clean := filepath.ToSlash(filepath.Clean(p))
	if idx := strings.Index(clean, ".specs/"); idx >= 0 {
		return clean[idx:]
	}
	return clean
}

// parseLogTimestamp parses the timestamp string from a log header. The
// project convention is "YYYY-MM-DD HH:MM"; we accept that plus a few
// looser shapes so older specs do not get dropped. On failure we return
// time.Now().UTC() so SeenAt is never zero — callers filtering by --since
// still get a meaningful comparison point.
func parseLogTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	layouts := []string{
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, parseErr := time.Parse(layout, raw); parseErr == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}

// extractTDDTokens scans a log entry body for the canonical
// "TDD: RED(...) → GREEN(...) → REFACTOR(...)" line and returns the
// three-token sequence. When the line is absent (e.g. a Batch header that
// only contains prose) the returned slice is empty — the parser still
// emits the record so the structural signal is preserved.
func extractTDDTokens(body string) []string {
	if tddRe.MatchString(body) {
		return []string{"RED", "GREEN", "REFACTOR"}
	}
	return nil
}

// splitFiles parses the right-hand side of a `- files: ...` sub-bullet.
// The convention in this project is a comma-separated list of paths each
// wrapped in backticks. We strip the backticks and surrounding whitespace
// so the resulting tokens are clean identifiers.
func splitFiles(rhs string) []string {
	parts := strings.Split(rhs, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		t = strings.Trim(t, "`")
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
