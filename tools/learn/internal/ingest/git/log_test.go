package git

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/sanitize"
)

// newTestSanitizer returns a sanitizer with the canonical aws_key rule used by
// the redaction TCs (REQ-4). Built directly here so tests don't depend on the
// project's config file existing on disk.
func newTestSanitizer(t *testing.T) *sanitize.Sanitizer {
	t.Helper()
	specs := []config.SecretPattern{
		{Kind: "aws_key", Pattern: `AKIA[0-9A-Z]{16}`},
	}
	san, err := sanitize.NewFromConfig(specs)
	if err != nil {
		t.Fatalf("build sanitizer: %v", err)
	}
	return san
}

// --- parseEntries unit tests (pure function, no git needed) ---

func TestParseEntries_Happy_ConventionalCommit(t *testing.T) {
	san := newTestSanitizer(t)
	raw := joinEntries(
		fixtureEntry("abc111", "2025-05-10T12:00:00+00:00", "feat(user): add cache layer", "",
			[]numstat{{"10", "0", "internal/usecases/user/get.go"}, {"5", "0", "internal/infrastructure/cache/redis.go"}},
		),
	)
	records, err := parseEntries(raw, san)
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}
	r := records[0]
	if r.Source != "abc111" {
		t.Errorf("Source: want abc111, got %q", r.Source)
	}
	if r.Kind != "commit" {
		t.Errorf("Kind: want commit, got %q", r.Kind)
	}
	want := []string{"feat", "user", "internal/infrastructure/cache", "internal/usecases/user"}
	if !equalSlices(r.Tokens, want) {
		t.Errorf("Tokens: want %v, got %v", want, r.Tokens)
	}
	if !strings.Contains(r.Body, "feat(user): add cache layer") {
		t.Errorf("Body missing subject: %q", r.Body)
	}
	if r.SeenAt.IsZero() {
		t.Errorf("SeenAt is zero")
	}
}

func TestParseEntries_Unconventional(t *testing.T) {
	san := newTestSanitizer(t)
	raw := fixtureEntry("def222", "2025-05-11T00:00:00+00:00", "just fix stuff", "",
		[]numstat{{"1", "1", "README.md"}},
	)
	records, err := parseEntries(raw, san)
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("want 1 record")
	}
	if records[0].Tokens[0] != "unconventional" {
		t.Errorf("Tokens[0]: want unconventional, got %q", records[0].Tokens[0])
	}
	if records[0].Tokens[1] != "" {
		t.Errorf("Tokens[1]: want empty scope, got %q", records[0].Tokens[1])
	}
}

func TestParseEntries_NoScope(t *testing.T) {
	san := newTestSanitizer(t)
	raw := fixtureEntry("ee333", "2025-05-11T00:00:00+00:00", "fix: handle nil", "",
		[]numstat{{"1", "1", "main.go"}},
	)
	records, err := parseEntries(raw, san)
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}
	if records[0].Tokens[0] != "fix" || records[0].Tokens[1] != "" {
		t.Errorf("Tokens: want [fix, \"\", ...], got %v", records[0].Tokens)
	}
}

func TestParseEntries_BinarySkipped(t *testing.T) {
	san := newTestSanitizer(t)
	// Capture stderr-bound warnings via the package logger.
	var buf bytes.Buffer
	origOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(origOutput)

	raw := joinEntries(
		fixtureEntry("aaa111", "2025-05-11T00:00:00+00:00", "feat(api): text only", "",
			[]numstat{{"1", "0", "a.go"}},
		),
		fixtureEntry("bbb222", "2025-05-11T00:00:00+00:00", "feat(api): binary blob", "",
			[]numstat{{"-", "-", "image.png"}},
		),
		fixtureEntry("ccc333", "2025-05-11T00:00:00+00:00", "feat(api): another", "",
			[]numstat{{"2", "0", "b.go"}},
		),
	)
	records, err := parseEntries(raw, san)
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("want 2 records (binary skipped), got %d", len(records))
	}
	for _, r := range records {
		if r.Source == "bbb222" {
			t.Errorf("binary commit was not skipped")
		}
	}
	if !strings.Contains(buf.String(), "bbb222") {
		t.Errorf("expected warn about bbb222 on stderr; got %q", buf.String())
	}
}

func TestParseEntries_SanitizerAppliedToBody(t *testing.T) {
	san := newTestSanitizer(t)
	raw := fixtureEntry("sec111", "2025-05-12T00:00:00+00:00",
		"chore: rotate keys",
		"old key AKIAABCDEFGHIJKLMNOP should be redacted",
		[]numstat{{"1", "0", "config/keys.go"}},
	)
	records, err := parseEntries(raw, san)
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}
	if strings.Contains(records[0].Body, "AKIAABCDEFGHIJKLMNOP") {
		t.Errorf("Body not sanitized: %q", records[0].Body)
	}
	if !strings.Contains(records[0].Body, "<REDACTED:aws_key>") {
		t.Errorf("Body missing redaction marker: %q", records[0].Body)
	}
}

func TestParseEntries_MaxDirsCap(t *testing.T) {
	san := newTestSanitizer(t)
	ns := []numstat{}
	for i := 0; i < 15; i++ {
		// 15 distinct top-level dirs to force the cap of 8.
		ns = append(ns, numstat{"1", "0", "pkg" + string(rune('a'+i)) + "/file.go"})
	}
	raw := fixtureEntry("cap111", "2025-05-12T00:00:00+00:00", "refactor(core): spread", "", ns)
	records, err := parseEntries(raw, san)
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}
	dirTokens := records[0].Tokens[2:]
	if len(dirTokens) != MaxDirsPerCommit {
		t.Errorf("dir tokens: want %d, got %d (%v)", MaxDirsPerCommit, len(dirTokens), dirTokens)
	}
	// alphabetical
	for i := 1; i < len(dirTokens); i++ {
		if dirTokens[i-1] >= dirTokens[i] {
			t.Errorf("dir tokens not sorted: %v", dirTokens)
			break
		}
	}
}

func TestParseEntries_EmptyInput(t *testing.T) {
	san := newTestSanitizer(t)
	records, err := parseEntries("", san)
	if err != nil {
		t.Fatalf("parseEntries empty: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("want 0 records, got %d", len(records))
	}
}

// --- ParseLog integration test (requires `git` on PATH) ---

func TestParseLog_HappyPath_IntegratesWithGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := initTestRepo(t)
	writeAndCommit(t, repo, "feat(user): add cache layer",
		map[string]string{
			"internal/usecases/user/get.go":          "package user\n",
			"internal/infrastructure/cache/redis.go": "package cache\n",
		}, "2025-05-10T12:00:00+00:00")

	writeAndCommit(t, repo, "fix(role): bound list size",
		map[string]string{"internal/usecases/role/list.go": "package role\n"},
		"2025-05-11T12:00:00+00:00")

	writeAndCommit(t, repo, "just fix stuff",
		map[string]string{"README.md": "hello\n"},
		"2025-05-12T12:00:00+00:00")

	san := newTestSanitizer(t)
	records, err := ParseLog(context.Background(), repo, Options{}, san)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("want 3 records, got %d", len(records))
	}
	kinds := map[string]bool{}
	for _, r := range records {
		kinds[r.Tokens[0]] = true
		if r.Kind != "commit" {
			t.Errorf("Kind: %q", r.Kind)
		}
		if r.Source == "" {
			t.Errorf("empty Source")
		}
	}
	for _, want := range []string{"feat", "fix", "unconventional"} {
		if !kinds[want] {
			t.Errorf("missing Tokens[0]==%q in results", want)
		}
	}
}

func TestParseLog_EmptyRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := initTestRepo(t)
	san := newTestSanitizer(t)
	records, err := ParseLog(context.Background(), repo, Options{}, san)
	if err != nil {
		t.Fatalf("ParseLog empty: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("want 0 records, got %d", len(records))
	}
}

func TestParseLog_SinceFilter(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := initTestRepo(t)
	old := time.Now().UTC().Add(-72 * time.Hour).Format("2006-01-02T15:04:05-0700")
	recent := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02T15:04:05-0700")

	writeAndCommit(t, repo, "feat: old", map[string]string{"old.go": "package x\n"}, old)
	writeAndCommit(t, repo, "feat: recent", map[string]string{"new.go": "package x\n"}, recent)

	san := newTestSanitizer(t)
	records, err := ParseLog(context.Background(), repo, Options{Since: "1 day ago"}, san)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("want 1 record (since=1d), got %d", len(records))
	}
	if !strings.Contains(records[0].Body, "recent") {
		t.Errorf("expected recent commit, got body %q", records[0].Body)
	}
}

func TestParseLog_RuntimeErrorOnBadRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	san := newTestSanitizer(t)
	dir := t.TempDir() // not a git repo
	_, err := ParseLog(context.Background(), dir, Options{}, san)
	if err == nil {
		t.Fatal("expected error from non-git dir")
	}
	var re *learnerr.RuntimeError
	if !errors.As(err, &re) {
		t.Errorf("want *learnerr.RuntimeError, got %T: %v", err, err)
	}
}

// --- helpers ---

type numstat struct {
	added, deleted, path string
}

// fixtureEntry builds a single git-log entry block in the exact format our
// parser expects. Format:
//
//	<sha>\x1f<iso-date>\x1f<subject>\x1f<body>\x1e
//	<added>\t<deleted>\t<path>
//	...
//
// where \x1e separates the header from numstat lines, \x1f separates header
// fields, and a blank line separates entries.
func fixtureEntry(sha, isoDate, subject, body string, ns []numstat) string {
	var b strings.Builder
	b.WriteString(sha)
	b.WriteString("\x1f")
	b.WriteString(isoDate)
	b.WriteString("\x1f")
	b.WriteString(subject)
	b.WriteString("\x1f")
	b.WriteString(body)
	b.WriteString("\x1e\n")
	for _, n := range ns {
		b.WriteString(n.added)
		b.WriteString("\t")
		b.WriteString(n.deleted)
		b.WriteString("\t")
		b.WriteString(n.path)
		b.WriteString("\n")
	}
	return b.String()
}

func joinEntries(entries ...string) string {
	return strings.Join(entries, "\n")
}

func equalSlices(a, b []string) bool {
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

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

func writeAndCommit(t *testing.T, repoDir, subject string, files map[string]string, isoDate string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(repoDir, path)
		if mkErr := os.MkdirAll(filepath.Dir(full), 0o755); mkErr != nil {
			t.Fatalf("mkdir: %v", mkErr)
		}
		if wrErr := os.WriteFile(full, []byte(content), 0o644); wrErr != nil {
			t.Fatalf("write: %v", wrErr)
		}
	}
	runGit(t, repoDir, "add", "-A")
	cmd := exec.Command("git", "-C", repoDir, "commit", "-q", "-m", subject)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+isoDate,
		"GIT_COMMITTER_DATE="+isoDate,
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("git commit: %v\n%s", runErr, out)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
