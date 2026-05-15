package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/sanitize"
)

// newTestSanitizer builds a Sanitizer with the project's default patterns.
// Tests rely on the aws_key pattern (AKIA…) being one of the defaults.
func newTestSanitizer(t *testing.T) *sanitize.Sanitizer {
	t.Helper()
	pats, buildErr := sanitize.BuiltinPatterns()
	if buildErr != nil {
		t.Fatalf("BuiltinPatterns failed: %v", buildErr)
	}
	return sanitize.New(pats)
}

// writeFile is a convenience helper for materializing fixture files inside
// t.TempDir(). It creates any missing parent directories.
func writeFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	abs := filepath.Join(root, rel)
	if mkErr := os.MkdirAll(filepath.Dir(abs), 0o755); mkErr != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(abs), mkErr)
	}
	if writeErr := os.WriteFile(abs, []byte(body), 0o644); writeErr != nil {
		t.Fatalf("WriteFile(%s): %v", abs, writeErr)
	}
	return abs
}

const feedbackQualityBody = `---
name: feedback-quality
description: Always prefer quality over velocity
metadata:
  type: feedback
---

Body with **markdown**.
`

const withWikilinksBody = `---
name: with-wikilinks
description: references other entries
metadata:
  type: reference
---

See [[live-skill]] for the canonical flow, but avoid [[deprecated-skill]].
`

const badYAMLBody = `---
name: bad-yaml
description: missing colon
metadata
  type: feedback
---

body
`

const noFrontmatterBody = `# Plain markdown

No frontmatter here.
`

const withAWSKeyBody = `---
name: with-aws-key
description: contains a leaked credential
metadata:
  type: feedback
---

This body leaks AKIAABCDEFGHIJKLMNOP and must be sanitized.
`

func TestParseFile_HappyPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeFile(t, dir, "feedback-quality.md", feedbackQualityBody)

	rec, parseErr := ParseFile(context.Background(), path, nil, newTestSanitizer(t))
	if parseErr != nil {
		t.Fatalf("ParseFile: %v", parseErr)
	}
	if rec.Kind != "memory-entry" {
		t.Errorf("Kind = %q, want %q", rec.Kind, "memory-entry")
	}
	if rec.Frontmatter.Name != "feedback-quality" {
		t.Errorf("Frontmatter.Name = %q", rec.Frontmatter.Name)
	}
	if rec.Frontmatter.Metadata.Type != "feedback" {
		t.Errorf("Frontmatter.Metadata.Type = %q", rec.Frontmatter.Metadata.Type)
	}
	if rec.Frontmatter.Description == "" {
		t.Errorf("Frontmatter.Description empty")
	}
	if !strings.Contains(rec.Body, "Body with **markdown**.") {
		t.Errorf("Body missing markdown content: %q", rec.Body)
	}
	if strings.Contains(rec.Body, "---") {
		t.Errorf("Body still contains frontmatter separator: %q", rec.Body)
	}
	wantTokens := []string{"feedback-quality", "feedback"}
	if !sliceEqual(rec.Tokens, wantTokens) {
		t.Errorf("Tokens = %v, want %v", rec.Tokens, wantTokens)
	}
	if len(rec.BrokenLinks) != 0 {
		t.Errorf("BrokenLinks = %v, want empty", rec.BrokenLinks)
	}
	if rec.SeenAt.IsZero() {
		t.Errorf("SeenAt is zero")
	}
	if !strings.HasSuffix(rec.Source, "feedback-quality.md") {
		t.Errorf("Source = %q, want suffix feedback-quality.md", rec.Source)
	}
}

func TestParseFile_BadYAML_UsageError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeFile(t, dir, "bad-yaml.md", badYAMLBody)

	_, parseErr := ParseFile(context.Background(), path, nil, newTestSanitizer(t))
	if parseErr == nil {
		t.Fatalf("expected error, got nil")
	}
	var ue *learnerr.UsageError
	if !errors.As(parseErr, &ue) {
		t.Fatalf("expected *learnerr.UsageError, got %T: %v", parseErr, parseErr)
	}
	// Error should cite line/column information to help the operator find the issue.
	if !strings.Contains(parseErr.Error(), "line") {
		t.Errorf("error %q should reference a line number", parseErr.Error())
	}
}

func TestParseFile_NoFrontmatter_UsageError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeFile(t, dir, "no-frontmatter.md", noFrontmatterBody)

	_, parseErr := ParseFile(context.Background(), path, nil, newTestSanitizer(t))
	if parseErr == nil {
		t.Fatalf("expected error, got nil")
	}
	var ue *learnerr.UsageError
	if !errors.As(parseErr, &ue) {
		t.Fatalf("expected *learnerr.UsageError, got %T: %v", parseErr, parseErr)
	}
	if !strings.Contains(strings.ToLower(parseErr.Error()), "missing frontmatter") {
		t.Errorf("error %q should mention missing frontmatter", parseErr.Error())
	}
}

func TestParseFile_FileNotFound_RuntimeError(t *testing.T) {
	t.Parallel()

	_, parseErr := ParseFile(context.Background(), filepath.Join(t.TempDir(), "missing.md"), nil, newTestSanitizer(t))
	if parseErr == nil {
		t.Fatalf("expected error, got nil")
	}
	var re *learnerr.RuntimeError
	if !errors.As(parseErr, &re) {
		t.Fatalf("expected *learnerr.RuntimeError, got %T: %v", parseErr, parseErr)
	}
}

func TestParseFile_Wikilinks_AndBrokenLinks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	path := writeFile(t, memDir, "with-wikilinks.md", withWikilinksBody)

	// Create a deprecated peer so [[deprecated-skill]] resolves to deprecated.
	deprecatedRoot := filepath.Join(memDir, "_deprecated")
	writeFile(t, deprecatedRoot, "deprecated-skill.md", "deprecated stub\n")

	rec, parseErr := ParseFile(
		context.Background(),
		path,
		[]string{deprecatedRoot},
		newTestSanitizer(t),
	)
	if parseErr != nil {
		t.Fatalf("ParseFile: %v", parseErr)
	}

	// Tokens must include [name, type, ...wikilink targets] in that order.
	wantTokens := []string{"with-wikilinks", "reference", "live-skill", "deprecated-skill"}
	if !sliceEqual(rec.Tokens, wantTokens) {
		t.Errorf("Tokens = %v, want %v", rec.Tokens, wantTokens)
	}

	// Exactly one broken link.
	if len(rec.BrokenLinks) != 1 {
		t.Fatalf("BrokenLinks = %v, want exactly 1 entry", rec.BrokenLinks)
	}
	if rec.BrokenLinks[0] != "deprecated-skill" {
		t.Errorf("BrokenLinks[0] = %q, want %q", rec.BrokenLinks[0], "deprecated-skill")
	}
}

func TestParseFile_SanitizerApplied(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeFile(t, dir, "with-aws-key.md", withAWSKeyBody)

	rec, parseErr := ParseFile(context.Background(), path, nil, newTestSanitizer(t))
	if parseErr != nil {
		t.Fatalf("ParseFile: %v", parseErr)
	}
	if strings.Contains(rec.Body, "AKIAABCDEFGHIJKLMNOP") {
		t.Errorf("Body still contains raw AWS key: %q", rec.Body)
	}
	if !strings.Contains(rec.Body, "<REDACTED:aws_key>") {
		t.Errorf("Body missing redaction marker: %q", rec.Body)
	}
}

func TestParseDir_HappyPath_SkipsIndexAndDeprecated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Valid entries.
	writeFile(t, dir, "feedback-quality.md", feedbackQualityBody)
	writeFile(t, dir, "with-aws-key.md", withAWSKeyBody)
	// Index must be skipped.
	writeFile(t, dir, "MEMORY.md", "# Index\n\n- placeholder\n")
	// _deprecated entries must be skipped.
	writeFile(t, dir, "_deprecated/old-thing.md", feedbackQualityBody)

	records, parseErr := ParseDir(context.Background(), dir, nil, newTestSanitizer(t))
	if parseErr != nil {
		t.Fatalf("ParseDir: %v", parseErr)
	}
	names := make([]string, 0, len(records))
	for _, r := range records {
		names = append(names, r.Frontmatter.Name)
	}
	sort.Strings(names)
	want := []string{"feedback-quality", "with-aws-key"}
	if !sliceEqual(names, want) {
		t.Errorf("ParseDir names = %v, want %v", names, want)
	}
}

func TestParseDir_BrokenFileLogsAndContinues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "feedback-quality.md", feedbackQualityBody)
	writeFile(t, dir, "no-frontmatter.md", noFrontmatterBody)

	records, parseErr := ParseDir(context.Background(), dir, nil, newTestSanitizer(t))
	if parseErr != nil {
		t.Fatalf("ParseDir returned error, expected to continue past per-file failure: %v", parseErr)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1 (broken file skipped)", len(records))
	}
	if records[0].Frontmatter.Name != "feedback-quality" {
		t.Errorf("unexpected record: %+v", records[0])
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
