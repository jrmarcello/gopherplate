// Git log harvester. Reads from a local git repository's commit history,
// one of the four ingest sources of the learning-loop harness (REQ-2 of
// .specs/learning-loop-harness.md). Each commit becomes a single gitRecord
// whose Tokens slice encodes the conventional-commit type/scope plus the
// unique directory prefixes of the changed files — downstream pattern miners
// consume Tokens to build ngrams without re-parsing the raw subject line.
//
// Determinism is deliberate: the parser reads a stable git log format using
// ASCII separators (US/RS), never tolerates arbitrary whitespace, and sorts
// directory tokens alphabetically so two runs on the same commit set produce
// byte-identical GitRecords. The sanitizer is applied to the commit body
// before it leaves this file, never after.

package learn

import (
	"bufio"
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

// maxGitDirsPerCommit caps the number of unique directory-prefix tokens emitted
// per commit. The cap bounds the Tokens slice length in pathological commits
// (e.g. mass refactors touching dozens of directories), which keeps the
// downstream ngram index bounded in memory.
const maxGitDirsPerCommit = 8

// fieldSep separates the per-commit header fields (sha, date, subject, body).
// It is U+001F INFORMATION SEPARATOR ONE — never appears in normal commit
// text, so it survives the round-trip through `git log --pretty=format:`.
const fieldSep = "\x1f"

// blockSep terminates the header block and introduces the numstat lines.
// U+001E INFORMATION SEPARATOR TWO — same rationale as fieldSep.
const blockSep = "\x1e"

// prettyFormat is the `git log --pretty=format:` template. Each commit
// renders as:
//
//	<sha>\x1f<author-date-iso-strict>\x1f<subject>\x1f<body>\x1e
//
// followed by the --numstat lines (one per changed file) and a blank line
// separating commits.
const prettyFormat = "%H" + fieldSep + "%aI" + fieldSep + "%s" + fieldSep + "%b" + blockSep

// conventionalRe matches a conventional-commit subject prefix and captures
// the type (group 1) and optional scope (group 2, without parens). Any
// subject that does not match yields the sentinel "unconventional" type and
// an empty scope (so Tokens length stays >= 2 — patterns rely on it).
var conventionalRe = regexp.MustCompile(`^(feat|fix|refactor|chore|docs|test|perf|build|ci|style|revert)(?:\(([^)]+)\))?:`)

// gitRecord is the harvested shape of a single commit. Fields are intentionally
// duplicated across the four ingest record types (specRecord, gitRecord,
// transcriptRecord, memoryRecord) so each can be tested in isolation; the
// contract is locked by the spec (REQ-2..REQ-5) and any divergence between
// them is a spec violation.
type gitRecord struct {
	Source string    // commit sha (full 40-char hex)
	Kind   string    // always "commit" for this record type
	Tokens []string  // [type, scope, dir1, dir2, ...] — see file doc
	SeenAt time.Time // author timestamp
	Body   string    // sanitized subject + body
}

// gitOptions tune the git log invocation. The zero value is valid: no since
// filter, unbounded commit count.
type gitOptions struct {
	// Since, when non-empty, becomes `git log --since=<value>`. Accepts any
	// value git accepts — relative ("7d", "30d", "1 week ago") or absolute
	// ISO timestamps.
	Since string

	// MaxCommits, when > 0, caps the number of commits returned. 0 means
	// unlimited (let git stream everything).
	MaxCommits int
}

// parseGitLog runs `git log` against repoDir and returns one gitRecord per
// commit. Binary diffs are detected via --numstat's "-\t-" marker and the
// entire commit is skipped with a structured warn on the package logger; this
// is TC-UC-11a from the spec. The Body field is sanitized via san before being
// returned, so callers can safely persist the gitRecord without re-running
// redaction.
//
// repoDir == "" defaults to the current working directory. Process errors
// (git missing, repo not a worktree, exit non-zero) are wrapped as
// *runtimeError so the CLI exits with code 2 per the REQ-22 contract.
func parseGitLog(ctx context.Context, repoDir string, opts gitOptions, san *sanitizer) ([]gitRecord, error) {
	if san == nil {
		return nil, runtimef("parseGitLog: sanitizer is required")
	}
	if repoDir == "" {
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			return nil, runtimef("parseGitLog: resolve cwd: %w", wdErr)
		}
		repoDir = wd
	}

	args := []string{
		"-C", repoDir,
		"log",
		"--numstat",
		"--date=iso-strict",
		"--pretty=format:" + prettyFormat,
	}
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}
	if opts.MaxCommits > 0 {
		args = append(args, "-n", itoaInt(opts.MaxCommits))
	}

	// args is built from constant flags + caller-supplied --since/--max-commits
	// values that pass through `time.ParseDuration` and `itoa`; no shell, no
	// user-typed strings reach the subprocess.
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // args is built from a controlled allowlist of git flags
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	out, runErr := cmd.Output()
	if runErr != nil {
		var exitErr *exec.ExitError
		stderrText := stderrBuf.String()
		// A repo that exists but has no commits yet exits 128 with a
		// stable error message. Treat it as "no records" rather than a
		// runtime failure — that mirrors what callers expect from a
		// freshly-initialized repository.
		if errors.As(runErr, &exitErr) && strings.Contains(stderrText, "does not have any commits yet") {
			return nil, nil
		}
		if errors.As(runErr, &exitErr) {
			return nil, runtimef("parseGitLog: git log exited %d: %s: %w", exitErr.ExitCode(), strings.TrimSpace(stderrText), runErr)
		}
		return nil, runtimef("parseGitLog: run git: %w", runErr)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}

	return parseGitEntries(string(out), san)
}

// parseGitEntries is the pure parser exposed to unit tests. It accepts the
// raw stdout of the git-log command (see prettyFormat) and returns one
// gitRecord per textual entry. Binary-diff commits are dropped with a warn on
// the package logger. The function never touches the filesystem or the
// process environment — every external concern lives in parseGitLog.
func parseGitEntries(raw string, san *sanitizer) ([]gitRecord, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	// Commit bodies can be large; raise the default 64KB line buffer cap.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	records := make([]gitRecord, 0, 16)

	var header strings.Builder
	var numstat []string
	inHeader := true

	flush := func() {
		defer func() {
			header.Reset()
			numstat = numstat[:0]
			inHeader = true
		}()

		text := strings.TrimSpace(header.String())
		if text == "" {
			return
		}

		rec, binary, parseErr := buildGitRecord(text, numstat, san)
		if parseErr != nil {
			log.Printf("parseGitEntries: skipping malformed entry: %v", parseErr)
			return
		}
		if binary {
			log.Printf("parseGitEntries: skipping commit %s (binary diff)", rec.Source)
			return
		}
		records = append(records, rec)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if inHeader {
			header.WriteString(line)
			header.WriteString("\n")
			if strings.Contains(line, blockSep) {
				inHeader = false
			}
			continue
		}
		if line == "" {
			flush()
			continue
		}
		numstat = append(numstat, line)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, runtimef("parseGitEntries: scan output: %w", scanErr)
	}
	flush()

	return records, nil
}

// buildGitRecord turns one header block + its numstat lines into a gitRecord.
// Returns (rec, binary=true, nil) when the commit must be skipped because
// any file in the diff is binary. Returns (zero, false, err) only for
// malformed input that should not have come out of git.
func buildGitRecord(header string, numstat []string, san *sanitizer) (gitRecord, bool, error) {
	headerText := strings.TrimRight(header, "\n")
	// Strip the trailing blockSep (and anything after it, which should be empty).
	if idx := strings.Index(headerText, blockSep); idx >= 0 {
		headerText = headerText[:idx]
	}

	fields := strings.SplitN(headerText, fieldSep, 4)
	if len(fields) < 4 {
		return gitRecord{}, false, runtimef("expected 4 header fields, got %d: %q", len(fields), headerText)
	}
	sha := strings.TrimSpace(fields[0])
	dateStr := strings.TrimSpace(fields[1])
	subject := fields[2]
	body := fields[3]

	seenAt, dateErr := parseGitDate(dateStr)
	if dateErr != nil {
		return gitRecord{}, false, runtimef("parse date %q: %w", dateStr, dateErr)
	}

	files, binary := parseNumstat(numstat)
	if binary {
		return gitRecord{Source: sha}, true, nil
	}

	commitType, scope := extractTypeScope(subject)
	dirs := dirTokens(files)
	tokens := make([]string, 0, 2+len(dirs))
	tokens = append(tokens, commitType, scope)
	tokens = append(tokens, dirs...)

	bodyText := subject
	if strings.TrimSpace(body) != "" {
		bodyText = subject + "\n\n" + body
	}
	bodyText = san.sanitize(bodyText)

	return gitRecord{
		Source: sha,
		Kind:   "commit",
		Tokens: tokens,
		SeenAt: seenAt,
		Body:   bodyText,
	}, false, nil
}

// parseNumstat returns the unique file paths from a numstat block, plus a
// binary flag set when any line uses the "-\t-" marker (git's notation for
// a binary diff). A nil/empty input returns an empty slice and binary=false
// — empty commits are still valid (a merge with no file changes).
func parseNumstat(lines []string) ([]string, bool) {
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		added, deleted, file := parts[0], parts[1], parts[2]
		if added == "-" && deleted == "-" {
			return nil, true
		}
		files = append(files, file)
	}
	return files, false
}

// extractTypeScope applies conventionalRe to subject. Non-matching subjects
// yield ("unconventional", "") so downstream ngram code can rely on a
// fixed-shape prefix of length 2 in every gitRecord's Tokens slice.
func extractTypeScope(subject string) (typ, scope string) {
	subject = strings.TrimSpace(subject)
	if m := conventionalRe.FindStringSubmatch(subject); m != nil {
		return m[1], m[2]
	}
	return "unconventional", ""
}

// dirTokens returns the unique parent directories of files, sorted
// alphabetically and capped at maxGitDirsPerCommit. Files at the repo root
// contribute the "." token. Empty input returns an empty slice (not nil) so
// the caller's append builds a stable Tokens shape.
func dirTokens(files []string) []string {
	if len(files) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		dir := path.Dir(file)
		if dir == "" {
			dir = "."
		}
		seen[dir] = struct{}{}
	}
	dirs := make([]string, 0, len(seen))
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	if len(dirs) > maxGitDirsPerCommit {
		dirs = dirs[:maxGitDirsPerCommit]
	}
	return dirs
}

// parseGitDate parses the iso-strict format git emits with --date=iso-strict.
// That format is RFC 3339 (e.g. "2025-05-10T12:00:00+00:00"), so we try
// time.RFC3339 first and fall back to a compact form without colons in the
// offset (which some git versions emit) as a defensive measure.
func parseGitDate(s string) (time.Time, error) {
	if t, parseErr := time.Parse(time.RFC3339, s); parseErr == nil {
		return t, nil
	}
	return time.Parse("2006-01-02T15:04:05-0700", s)
}

// itoaInt is a tiny helper that avoids pulling in strconv for the single
// integer-to-string conversion parseGitLog needs. Keeping the import surface
// small makes the file easier to audit.
func itoaInt(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
