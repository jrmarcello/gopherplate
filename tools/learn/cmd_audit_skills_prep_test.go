package learn

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeAuditSkill seeds <dir>/<sub>/SKILL.md with the supplied frontmatter
// and body content. Returns the file path so tests can stat for mtime.
func writeAuditSkill(t *testing.T, dir, sub, frontmatter, body string) string {
	t.Helper()
	full := filepath.Join(dir, sub)
	if mkErr := os.MkdirAll(full, 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	path := filepath.Join(full, "SKILL.md")
	content := "---\n" + frontmatter + "---\n" + body
	if writeErr := os.WriteFile(path, []byte(content), 0o644); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	return path
}

// TC-UC-37 / TC-E2E-12a: 2-skill fixture produces 2 entries in the JSON
// output and leaves the source files' mtimes untouched.
func TestRunAuditSkillsPrep_twoSkillFixture_jsonAndNoMutation(t *testing.T) {
	dir := t.TempDir()
	p1 := writeAuditSkill(t, dir, "alpha",
		"name: alpha\ndescription: do alpha\nargument_hint: <foo>\nuser_invocable: true\ntags: x,y\n",
		"Alpha body — short.\n",
	)
	p2 := writeAuditSkill(t, dir, "beta",
		"name: beta\ndescription: do beta\nuser_invocable: false\n",
		strings.Repeat("z", 2000),
	)

	// Force the mtimes to a known past value so we can detect any writes.
	pastTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, p := range []string{p1, p2} {
		if chErr := os.Chtimes(p, pastTime, pastTime); chErr != nil {
			t.Fatalf("chtimes %q: %v", p, chErr)
		}
	}

	var buf bytes.Buffer
	runErr := runAuditSkillsPrep(auditSkillsPrepOpts{
		SkillsDir:    dir,
		MaxBodyChars: 50,
		Stdout:       &buf,
	})
	if runErr != nil {
		t.Fatalf("runAuditSkillsPrep: %v", runErr)
	}

	var got auditSkillsPrepOutput
	if decErr := json.Unmarshal(buf.Bytes(), &got); decErr != nil {
		t.Fatalf("decode JSON: %v; stdout=%s", decErr, buf.String())
	}
	if len(got.Skills) != 2 {
		t.Fatalf("want 2 skills, got %d (output=%s)", len(got.Skills), buf.String())
	}
	// Determine ordering by Path (alpha before beta lexicographically).
	if !strings.HasSuffix(got.Skills[0].Path, "alpha/SKILL.md") {
		t.Errorf("expected alpha first; got %q", got.Skills[0].Path)
	}
	if got.Skills[0].Name != "alpha" || got.Skills[0].Description != "do alpha" {
		t.Errorf("alpha fields wrong: %+v", got.Skills[0])
	}
	if !got.Skills[0].UserInvocable {
		t.Errorf("alpha user_invocable: want true, got false")
	}
	if got.Skills[1].UserInvocable {
		t.Errorf("beta user_invocable: want false, got true")
	}
	if got.Skills[1].BodySizeBytes < 2000 {
		t.Errorf("beta body size: want >=2000, got %d", got.Skills[1].BodySizeBytes)
	}
	if len(got.Skills[1].BodyExcerpt) > 60 { // 50 chars + UTF-8 slack
		t.Errorf("beta excerpt too long: %d chars", len(got.Skills[1].BodyExcerpt))
	}

	// Mtimes unchanged (TC-E2E-12a).
	for _, p := range []string{p1, p2} {
		info, statErr := os.Stat(p)
		if statErr != nil {
			t.Fatalf("stat: %v", statErr)
		}
		if !info.ModTime().Equal(pastTime) {
			t.Errorf("expected mtime unchanged for %q; want %v, got %v", p, pastTime, info.ModTime())
		}
	}
}

// Edge: a _deprecated/ subtree is skipped entirely.
func TestRunAuditSkillsPrep_skipsDeprecated(t *testing.T) {
	dir := t.TempDir()
	writeAuditSkill(t, dir, "live",
		"name: live\ndescription: d\n", "body\n")
	writeAuditSkill(t, dir, "_deprecated/old",
		"name: old\ndescription: d\n", "body\n")

	var buf bytes.Buffer
	runErr := runAuditSkillsPrep(auditSkillsPrepOpts{SkillsDir: dir, MaxBodyChars: 100, Stdout: &buf})
	if runErr != nil {
		t.Fatalf("run: %v", runErr)
	}
	var got auditSkillsPrepOutput
	if decErr := json.Unmarshal(buf.Bytes(), &got); decErr != nil {
		t.Fatalf("decode: %v", decErr)
	}
	if len(got.Skills) != 1 {
		t.Fatalf("want 1 skill (live only), got %d", len(got.Skills))
	}
	if !strings.Contains(got.Skills[0].Path, "live") {
		t.Errorf("unexpected path %q", got.Skills[0].Path)
	}
}
