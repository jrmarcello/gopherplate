// Audit log: appends one JSONL line per applied / rolled-back learning-loop
// decision to <learningDir>/audit.jsonl. The file is the authoritative trail
// for the auto-apply policy adopted by /learn-refine (REQ-9 / REQ-9a) —
// every move into _deprecated/ AND every restoration via /learn-rollback
// produces exactly one auditEntry here.
//
// File handling is best-effort: callers should NOT bubble a failed append
// out as a runtime error if the underlying mutation already succeeded. The
// audit log is observability, not the source of truth (the source of truth
// is the `decisions` SQL row + the filesystem state).
package learn

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// auditFileName is the canonical basename inside the learning directory.
const auditFileName = "audit.jsonl"

// auditEntry is one JSONL line.
//
// CandidateSignature and Rationale are optional context propagated from the
// originating `decisions` row. DeprecatedPath is the post-move location
// (empty for actions that don't move files). MergedInto is the canonical
// replacement path captured in the move header (REQ-10).
type auditEntry struct {
	Timestamp          string `json:"timestamp"`
	DecisionID         int64  `json:"decision_id"`
	Action             string `json:"action"` // "applied" | "rolled-back"
	SourcePath         string `json:"source_path"`
	DeprecatedPath     string `json:"deprecated_path,omitempty"`
	MergedInto         string `json:"merged_into,omitempty"`
	CandidateSignature string `json:"candidate_signature,omitempty"`
	Rationale          string `json:"rationale,omitempty"`
}

// fileMu serializes appends from concurrent goroutines in-process. Multi-
// process append serialization is the OS's job — append-mode writes of
// short JSON lines are atomic on POSIX up to PIPE_BUF (≥ 4 KiB), which our
// entries fit inside.
var fileMu sync.Mutex

// auditPath resolves the canonical audit file path inside learningDir.
func auditPath(learningDir string) string {
	return filepath.Join(learningDir, auditFileName)
}

// appendAudit serializes e to JSON and appends a single line to <learningDir>/audit.jsonl.
// The directory is created with 0o700 if missing (matching init.go's learning-dir
// policy). Failures wrap as *learn.runtimeError; callers may choose to log
// rather than bubble (the audit log is observability, not the source of truth).
func appendAudit(learningDir string, e auditEntry) error {
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	line, marshalErr := json.Marshal(&e)
	if marshalErr != nil {
		return runtimef("audit: marshal entry: %w", marshalErr)
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	if mkErr := os.MkdirAll(learningDir, 0o700); mkErr != nil {
		return runtimef("audit: create learning dir %q: %w", learningDir, mkErr)
	}
	path := auditPath(learningDir)
	f, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // operator-owned learning dir
	if openErr != nil {
		return runtimef("audit: open %q: %w", path, openErr)
	}
	defer func() { _ = f.Close() }()
	if _, writeErr := f.Write(append(line, '\n')); writeErr != nil {
		return runtimef("audit: write %q: %w", path, writeErr)
	}
	return nil
}

// readAudit returns every auditEntry in audit.jsonl. Missing file returns nil, nil
// (zero entries is a valid state — a fresh learning dir has no audit log).
// Malformed lines are skipped silently; callers expecting strict decode
// should validate the slice themselves.
func readAudit(learningDir string) ([]auditEntry, error) {
	path := auditPath(learningDir)
	f, openErr := os.Open(path) //nolint:gosec // operator-owned learning dir
	if openErr != nil {
		if errors.Is(openErr, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, runtimef("audit: open %q: %w", path, openErr)
	}
	defer func() { _ = f.Close() }()

	var out []auditEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var e auditEntry
		if unmarshalErr := json.Unmarshal(raw, &e); unmarshalErr != nil {
			continue // skip malformed line
		}
		out = append(out, e)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return out, runtimef("audit: scan %q: %w", path, scanErr)
	}
	return out, nil
}

// findAuditByDecisionID returns the LATEST entry for the given decision id (a
// decision can appear twice: once as `applied`, once as `rolled-back`). Used
// by /learn-rollback to locate the deprecated file to restore. Returns
// nil, nil when no entry matches.
func findAuditByDecisionID(learningDir string, id int64) (*auditEntry, error) {
	entries, readErr := readAudit(learningDir)
	if readErr != nil {
		return nil, readErr
	}
	var found *auditEntry
	for i := range entries {
		if entries[i].DecisionID == id {
			found = &entries[i]
		}
	}
	return found, nil
}

// learningDirFromDB is a convenience that derives the learning dir from the
// DB path passed by every subcommand (typically .claude/learning/db.sqlite).
// Empty input returns the empty string; callers should default-handle.
func learningDirFromDB(dbPath string) string {
	if dbPath == "" {
		return ""
	}
	return filepath.Dir(dbPath)
}
