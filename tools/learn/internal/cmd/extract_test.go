package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/pattern"
)

// writeF is a local helper analogous to writeFile but available here without
// import cycles when this test file is moved.
func writeF(t *testing.T, path, content string) {
	t.Helper()
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
		t.Fatalf("mkdir for %q: %v", path, mkErr)
	}
	if writeErr := os.WriteFile(path, []byte(content), 0o644); writeErr != nil {
		t.Fatalf("write %q: %v", path, writeErr)
	}
}

// minimalConfig returns a *config.Config with the supplied min_pattern_freq.
// All current callers pass 2 — the parameter is kept so future tests can
// inject a different floor without a helper-rename churn.
//
//nolint:unparam // min is intentionally parameterizable for future test fixtures
func minimalConfig(min int) *config.Config {
	cfg := config.DefaultConfig()
	cfg.MinPatternFreq = min
	return cfg
}

// makeTranscriptJSONL builds a JSONL transcript with the given tool-use
// sequence at distinct timestamps. tools is the chronological list of tool
// names emitted by the assistant.
func makeTranscriptJSONL(start time.Time, tools []string) string {
	var b strings.Builder
	for i, tool := range tools {
		ts := start.Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339)
		b.WriteString(`{"type":"assistant","timestamp":"`)
		b.WriteString(ts)
		b.WriteString(`","message":{"role":"assistant","content":[{"type":"tool_use","name":"`)
		b.WriteString(tool)
		b.WriteString(`","input":{}}]}}` + "\n")
	}
	return b.String()
}

// setupExtractEnv builds a temp dir tree with .specs/, transcripts/, memory/,
// and a learning dir (db + config). Returns paths for runExtract opts.
//
//nolint:unparam // root retained for future callers that need the temp root
func setupExtractEnv(t *testing.T) (root, dbPath, specsDir, transcriptsDir, memoryDir, outPath string) {
	t.Helper()
	root = t.TempDir()
	learningDir := filepath.Join(root, "learning")
	if runErr := runInit(initOpts{LearningDir: learningDir}); runErr != nil {
		t.Fatalf("setup: runInit: %v", runErr)
	}
	dbPath = filepath.Join(learningDir, "db.sqlite")
	specsDir = filepath.Join(root, ".specs")
	transcriptsDir = filepath.Join(root, "transcripts")
	memoryDir = filepath.Join(root, "memory")
	for _, d := range []string{specsDir, transcriptsDir, memoryDir} {
		if mkErr := os.MkdirAll(d, 0o755); mkErr != nil {
			t.Fatalf("setup: mkdir %s: %v", d, mkErr)
		}
	}
	outPath = filepath.Join(root, "candidates.jsonl")
	return root, dbPath, specsDir, transcriptsDir, memoryDir, outPath
}

// readCandidatesFile reads candidates.jsonl and returns each line as a
// strict-parsed Candidate (TC-UC-08a).
func readCandidatesFile(t *testing.T, path string) []*pattern.Candidate {
	t.Helper()
	raw, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read candidates: %v", readErr)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte{'\n'})
	out := make([]*pattern.Candidate, 0, len(lines))
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		c, unmarshalErr := pattern.UnmarshalStrict(line)
		if unmarshalErr != nil {
			t.Fatalf("strict unmarshal line %d (%q): %v", i, string(line), unmarshalErr)
		}
		out = append(out, c)
	}
	return out
}

// TC-UC-07: happy path — 2 transcripts repeating the same tool sequence
// produce at least one tool-sequence candidate whose Signature contains the
// repeated tokens.
func TestRunExtract_HappyPath_ToolSequence(t *testing.T) {
	_, dbPath, specsDir, transcriptsDir, memoryDir, outPath := setupExtractEnv(t)

	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)

	// Two transcript files each repeating Read,Edit,Bash twice (total = 4
	// occurrences of the 3-gram across both files).
	for i, sess := range []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	} {
		path := filepath.Join(transcriptsDir, sess+".jsonl")
		writeF(t, path, makeTranscriptJSONL(now.Add(time.Duration(i)*time.Minute),
			[]string{"Read", "Edit", "Bash", "Read", "Edit", "Bash"}))
	}

	// Add a spec file with at least one completed task so file-sequence path
	// runs without panic. Not required to produce a candidate.
	writeF(t, filepath.Join(specsDir, "demo.md"), "## Status: IN_PROGRESS\n\n- [x] TASK-1 do thing\n    - files: `a.go`, `b.go`\n")

	stderr := &bytes.Buffer{}
	runErr := runExtract(context.Background(), extractOpts{
		DBPath:         dbPath,
		SpecsDir:       specsDir,
		TranscriptsDir: transcriptsDir,
		MemoryDir:      memoryDir,
		Out:            outPath,
		Config:         minimalConfig(2),
		Stderr:         stderr,
		Now:            func() time.Time { return now.Add(time.Hour) },
	})
	if runErr != nil {
		t.Fatalf("runExtract: %v (stderr=%s)", runErr, stderr.String())
	}

	cands := readCandidatesFile(t, outPath)
	if len(cands) == 0 {
		t.Fatalf("expected at least one candidate, got 0 (stderr=%s)", stderr.String())
	}

	// Must contain a tool-sequence candidate with signature mentioning Read & Edit
	found := false
	for _, c := range cands {
		if c.Kind == pattern.KindToolSequence &&
			strings.Contains(c.Signature, "Read") &&
			strings.Contains(c.Signature, "Edit") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no tool-sequence candidate with Read+Edit: %+v", cands)
	}
}

// TC-UC-08: --since filter excludes records outside the window.
func TestRunExtract_SinceFilter(t *testing.T) {
	_, dbPath, specsDir, transcriptsDir, memoryDir, outPath := setupExtractEnv(t)

	now := time.Now().UTC()
	old := now.Add(-30 * 24 * time.Hour)

	// Old transcript with a strong repeating pattern.
	writeF(t, filepath.Join(transcriptsDir, "cccccccccccccccccccccccccccccccc.jsonl"),
		makeTranscriptJSONL(old, []string{"Read", "Edit", "Read", "Edit", "Read", "Edit"}))

	stderr := &bytes.Buffer{}
	runErr := runExtract(context.Background(), extractOpts{
		DBPath:         dbPath,
		SpecsDir:       specsDir,
		TranscriptsDir: transcriptsDir,
		MemoryDir:      memoryDir,
		Out:            outPath,
		Since:          7 * 24 * time.Hour,
		Config:         minimalConfig(2),
		Stderr:         stderr,
		Now:            func() time.Time { return now },
	})
	if runErr != nil {
		t.Fatalf("runExtract: %v (stderr=%s)", runErr, stderr.String())
	}

	cands := readCandidatesFile(t, outPath)
	// All transcript records are older than 7d → filtered out. Expect no
	// tool-sequence candidate.
	for _, c := range cands {
		if c.Kind == pattern.KindToolSequence {
			t.Fatalf("expected no tool-sequence candidate after --since=7d filter, got %+v", c)
		}
	}
}

// TC-UC-08a (REQ-2a): every emitted line is strict-decodable.
// Reused via readCandidatesFile in the happy-path test; this test is an
// explicit assertion on schema purity.
func TestRunExtract_StrictUnmarshalable(t *testing.T) {
	_, dbPath, specsDir, transcriptsDir, memoryDir, outPath := setupExtractEnv(t)

	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)

	writeF(t, filepath.Join(transcriptsDir, "dddddddddddddddddddddddddddddddd.jsonl"),
		makeTranscriptJSONL(now, []string{"Read", "Edit", "Read", "Edit"}))

	runErr := runExtract(context.Background(), extractOpts{
		DBPath:         dbPath,
		SpecsDir:       specsDir,
		TranscriptsDir: transcriptsDir,
		MemoryDir:      memoryDir,
		Out:            outPath,
		Config:         minimalConfig(2),
		Now:            func() time.Time { return now.Add(time.Hour) },
	})
	if runErr != nil {
		t.Fatalf("runExtract: %v", runErr)
	}

	// Actually parse every line via UnmarshalStrict (not just os.Stat). A
	// mutant that writes an empty file or one with unknown fields would be
	// caught here — file existence alone proved nothing about schema
	// fidelity. We require at least one candidate so the strict decoder
	// has something to validate.
	cands := readCandidatesFile(t, outPath)
	if len(cands) == 0 {
		t.Fatalf("expected at least one candidate to exercise strict unmarshal, got 0")
	}
	for i, c := range cands {
		if c.Signature == "" {
			t.Fatalf("candidate %d has empty signature after strict decode", i)
		}
		if c.Frequency < 1 {
			t.Fatalf("candidate %d has invalid frequency %d", i, c.Frequency)
		}
	}
}

// TC-UC-35c (REQ-5 edge): empty sources → empty candidates.jsonl, no error.
func TestRunExtract_NoSources_EmptyOutput(t *testing.T) {
	_, dbPath, specsDir, transcriptsDir, memoryDir, outPath := setupExtractEnv(t)
	// All source dirs exist but are empty.

	stderr := &bytes.Buffer{}
	runErr := runExtract(context.Background(), extractOpts{
		DBPath:         dbPath,
		SpecsDir:       specsDir,
		TranscriptsDir: transcriptsDir,
		MemoryDir:      memoryDir,
		Out:            outPath,
		Config:         minimalConfig(2),
		Stderr:         stderr,
		Now:            time.Now,
	})
	if runErr != nil {
		t.Fatalf("runExtract with empty sources should not error: %v", runErr)
	}

	info, statErr := os.Stat(outPath)
	if statErr != nil {
		t.Fatalf("expected output file to be created even when empty: %v", statErr)
	}
	if info.Size() != 0 {
		raw, _ := os.ReadFile(outPath)
		t.Fatalf("expected empty candidates.jsonl, got %d bytes: %q", info.Size(), raw)
	}
}

// Determinism: two runs on identical input produce byte-identical output.
func TestRunExtract_Deterministic(t *testing.T) {
	_, dbPath, specsDir, transcriptsDir, memoryDir, outPath := setupExtractEnv(t)
	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)

	for i, sess := range []string{
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		"ffffffffffffffffffffffffffffffff",
		"gggggggggggggggggggggggggggggggg",
	} {
		writeF(t, filepath.Join(transcriptsDir, sess+".jsonl"),
			makeTranscriptJSONL(now.Add(time.Duration(i)*time.Minute),
				[]string{"Read", "Edit", "Bash", "Read", "Edit", "Bash"}))
	}

	runOnce := func(out string) []byte {
		t.Helper()
		runErr := runExtract(context.Background(), extractOpts{
			DBPath:         dbPath,
			SpecsDir:       specsDir,
			TranscriptsDir: transcriptsDir,
			MemoryDir:      memoryDir,
			Out:            out,
			Config:         minimalConfig(2),
			Now:            func() time.Time { return now.Add(time.Hour) },
		})
		if runErr != nil {
			t.Fatalf("runExtract: %v", runErr)
		}
		raw, readErr := os.ReadFile(out)
		if readErr != nil {
			t.Fatalf("read %s: %v", out, readErr)
		}
		return raw
	}

	outA := outPath
	outB := outPath + ".second"
	a := runOnce(outA)
	b := runOnce(outB)
	if !bytes.Equal(a, b) {
		t.Fatalf("non-deterministic output:\nA=%q\nB=%q", a, b)
	}
	if len(a) == 0 {
		t.Fatalf("expected non-empty output")
	}
}

// TC-E2E-07 (REQ-4): a spec body containing a secret has it redacted in
// candidates.jsonl. The literal must never appear.
func TestRunExtract_RedactsSecretsInSpecBody(t *testing.T) {
	_, dbPath, specsDir, transcriptsDir, memoryDir, outPath := setupExtractEnv(t)

	const awsKey = "AKIAIOSFODNN7EXAMPLE"

	// Build a spec with several tasks referencing common file tokens so
	// file-sequence n-grams cross the min-freq threshold. The secret appears
	// in the body of each task.
	body := `## Status: IN_PROGRESS

- [x] TASK-1 first
    - files: ` + "`internal/a.go`" + `, ` + "`internal/b.go`" + `
    - leaks: ` + awsKey + `

- [x] TASK-2 second
    - files: ` + "`internal/a.go`" + `, ` + "`internal/b.go`" + `
    - leaks: ` + awsKey + `

- [x] TASK-3 third
    - files: ` + "`internal/a.go`" + `, ` + "`internal/b.go`" + `
    - leaks: ` + awsKey + `
`
	writeF(t, filepath.Join(specsDir, "leak.md"), body)

	now := time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC)
	runErr := runExtract(context.Background(), extractOpts{
		DBPath:         dbPath,
		SpecsDir:       specsDir,
		TranscriptsDir: transcriptsDir,
		MemoryDir:      memoryDir,
		Out:            outPath,
		Config:         minimalConfig(2),
		Now:            func() time.Time { return now },
	})
	if runErr != nil {
		t.Fatalf("runExtract: %v", runErr)
	}

	raw, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("read out: %v", readErr)
	}
	if bytes.Contains(raw, []byte(awsKey)) {
		t.Fatalf("candidates.jsonl contains raw AWS key %q:\n%s", awsKey, raw)
	}
	// Sanity: tokens themselves won't typically contain the key (the spec
	// parser puts files in Tokens, not the body), but the redaction sentinel
	// would appear if the key had leaked anywhere — we don't require it.
}
