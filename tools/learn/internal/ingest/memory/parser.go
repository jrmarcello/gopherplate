// Package memory parses `memory/*.md` files (Claude-style memory entries with
// YAML frontmatter and wikilink references) into Records consumable by the
// learning-loop ingest pipeline.
//
// REQ-2 (memory source): every non-index `.md` file under a memory root is one
// Record. The Tokens slice carries the entry name, its declared type, and the
// targets of every `[[wikilink]]` so downstream similarity (REQ-3) can cluster
// related entries even when their bodies diverge stylistically.
//
// REQ-19a (broken-link audit): when a wikilink target resolves under one of
// the supplied `deprecatedRoots` (e.g. `memory/_deprecated/`,
// `.claude/skills/_deprecated/<name>/SKILL.md`) the target is recorded in
// BrokenLinks. Callers surface these in audit reports.
package memory

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/sanitize"
)

// Kind is the constant Record.Kind value for memory entries. It exists so
// downstream consumers can switch on a typed constant instead of a free-form
// string literal.
const Kind = "memory-entry"

// indexFile is the well-known memory index file name. ParseDir skips it
// because it is a hand-curated table of contents, not a memory entry.
const indexFile = "MEMORY.md"

// deprecatedDirName is the sentinel directory under a memory root that
// contains retired entries. ParseDir refuses to ingest its contents.
const deprecatedDirName = "_deprecated"

// wikilinkRe matches `[[kebab-case-slug]]` references in memory bodies.
// Slugs must start with `[a-z0-9]` so accidental matches inside code spans
// (e.g. `[[1]]` array literals in YAML) are still allowed but trivial
// (numeric) tokens still count — keeping the regex permissive here is
// intentional, the broken-link audit will surface anything weird.
var wikilinkRe = regexp.MustCompile(`\[\[([a-z0-9][a-z0-9-]*)\]\]`)

// frontmatterDelim is the literal YAML frontmatter delimiter as written by
// every editor that supports the format.
const frontmatterDelim = "---"

// Record is one parsed memory entry. It is the contract shared with the
// store layer; field order, types, and zero-value semantics are locked.
type Record struct {
	// Source is the path of the file as it should appear in store rows.
	// Callers may pass an absolute path to ParseFile; the field reflects
	// whatever was supplied verbatim — normalisation is the caller's job.
	Source string
	// Kind is always memory.Kind ("memory-entry").
	Kind string
	// Tokens carries the entry name, its declared type, and every
	// wikilink target (in document order). It feeds the ngram clustering
	// used by REQ-3.
	Tokens []string
	// SeenAt is the file's mtime at parse time.
	SeenAt time.Time
	// Body is the sanitized markdown body (everything after the closing
	// frontmatter delimiter). The leading blank line is trimmed; trailing
	// whitespace is preserved.
	Body string
	// Frontmatter is the parsed YAML header.
	Frontmatter Frontmatter
	// BrokenLinks lists wikilink targets that resolve under one of the
	// deprecated roots passed to ParseFile (REQ-19a). Empty when no broken
	// link is found or when no deprecatedRoots are supplied.
	BrokenLinks []string
}

// Frontmatter is the structured YAML header at the top of every memory
// entry. Unknown fields are ignored (yaml.v3's default behavior) so the
// schema can grow without breaking the parser.
type Frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Metadata    struct {
		Type string `yaml:"type"`
	} `yaml:"metadata"`
}

// ParseFile reads one memory markdown file, parses its YAML frontmatter and
// body, applies the sanitizer to Body and Tokens, and (if deprecatedRoots
// are supplied) audits every wikilink target against them.
//
// Errors are typed for the learnerr exit-code contract (REQ-22): file IO
// failures surface as *learnerr.RuntimeError; format violations (missing
// frontmatter, malformed YAML) surface as *learnerr.UsageError with the
// failing line cited when the YAML library exposes one.
//
// ctx is honored for cancellation between sub-steps but the IO is
// otherwise small enough that fine-grained cancellation would add more
// complexity than value.
func ParseFile(ctx context.Context, path string, deprecatedRoots []string, san *sanitize.Sanitizer) (*Record, error) {
	if san == nil {
		return nil, learnerr.Usagef("memory.ParseFile: sanitizer is nil (REQ-4 violation)")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, learnerr.Runtimef("memory.ParseFile: %w", ctxErr)
	}

	raw, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, learnerr.Runtimef("memory.ParseFile: read %q: %w", path, readErr)
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		return nil, learnerr.Runtimef("memory.ParseFile: stat %q: %w", path, statErr)
	}

	fm, body, splitErr := extractFrontmatter(raw)
	if splitErr != nil {
		return nil, learnerr.Usagef("memory.ParseFile: %s: %w", path, splitErr)
	}

	wikilinks := extractWikilinks(body)

	tokens := make([]string, 0, 2+len(wikilinks))
	if fm.Name != "" {
		tokens = append(tokens, fm.Name)
	}
	if fm.Metadata.Type != "" {
		tokens = append(tokens, fm.Metadata.Type)
	}
	tokens = append(tokens, wikilinks...)

	// Sanitize every Token (defensive — a leaked secret could in principle
	// reach a wikilink target slug, even if the regex makes that unlikely).
	for i, tok := range tokens {
		tokens[i] = san.Sanitize(tok)
	}

	brokenLinks := auditBrokenLinks(wikilinks, deprecatedRoots)

	return &Record{
		Source:      path,
		Kind:        Kind,
		Tokens:      tokens,
		SeenAt:      info.ModTime(),
		Body:        san.Sanitize(strings.TrimPrefix(body, "\n")),
		Frontmatter: fm,
		BrokenLinks: brokenLinks,
	}, nil
}

// ParseDir walks `<dir>/*.md` (non-recursive — memory is a flat namespace),
// skipping `MEMORY.md` and anything under `_deprecated/`. Per-file errors
// are logged via log/slog and the walk continues; the only error returned
// is a directory-listing failure.
func ParseDir(ctx context.Context, dir string, deprecatedRoots []string, san *sanitize.Sanitizer) ([]Record, error) {
	if san == nil {
		return nil, learnerr.Usagef("memory.ParseDir: sanitizer is nil (REQ-4 violation)")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, learnerr.Runtimef("memory.ParseDir: %w", ctxErr)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return nil, learnerr.Runtimef("memory.ParseDir: readdir %q: %w", dir, readErr)
	}

	records := make([]Record, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			// _deprecated is the only directory we deliberately skip; we
			// also do not recurse into any subdir because memory is a flat
			// namespace.
			continue
		}
		name := entry.Name()
		if name == indexFile {
			continue
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if isDeprecated(filepath.Join(dir, name)) {
			continue
		}

		path := filepath.Join(dir, name)
		rec, parseErr := ParseFile(ctx, path, deprecatedRoots, san)
		if parseErr != nil {
			slog.WarnContext(ctx, "memory.ParseDir: skipping file",
				"path", path,
				"error", parseErr.Error(),
			)
			continue
		}
		records = append(records, *rec)
	}
	return records, nil
}

// isDeprecated reports whether path lives under a `_deprecated/` segment.
// It is a pure path check — no IO — so callers can use it in walks.
func isDeprecated(path string) bool {
	cleaned := filepath.ToSlash(filepath.Clean(path))
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == deprecatedDirName {
			return true
		}
	}
	return false
}

// extractFrontmatter splits raw into (parsed frontmatter, body). The
// expected layout is `---\n<yaml>\n---\n<body>`. Any deviation surfaces as
// an error suitable for wrapping in *learnerr.UsageError. The returned
// body keeps its trailing newlines; the leading newline after the closing
// delimiter is trimmed by the caller.
func extractFrontmatter(raw []byte) (Frontmatter, string, error) {
	text := string(raw)
	if !strings.HasPrefix(text, frontmatterDelim) {
		return Frontmatter{}, "", errors.New("missing frontmatter: file does not start with '---'")
	}

	// Find the closing delimiter. It must appear on its own line; we anchor
	// on "\n---" to avoid matching the opening delimiter or in-body content.
	rest := text[len(frontmatterDelim):]
	// rest must start with a newline so that the YAML body begins on a fresh
	// line. A file that begins with "---<eof>" is malformed.
	if !strings.HasPrefix(rest, "\n") {
		return Frontmatter{}, "", errors.New("missing frontmatter: opening delimiter not followed by newline")
	}
	rest = rest[1:]

	closeIdx := strings.Index(rest, "\n"+frontmatterDelim)
	if closeIdx < 0 {
		return Frontmatter{}, "", errors.New("missing frontmatter: closing '---' delimiter not found")
	}
	yamlBlock := rest[:closeIdx]
	body := rest[closeIdx+len("\n"+frontmatterDelim):]
	// Trim the newline right after the closing delimiter, if present, so the
	// caller does not see a leading blank line.
	body = strings.TrimPrefix(body, "\n")

	var fm Frontmatter
	if unmarshalErr := yaml.Unmarshal([]byte(yamlBlock), &fm); unmarshalErr != nil {
		// yaml.v3 errors typically embed "line N:"; surface them verbatim so
		// the operator can jump to the offending line.
		return Frontmatter{}, "", fmt.Errorf("invalid YAML frontmatter (line/column from yaml): %w", unmarshalErr)
	}
	return fm, body, nil
}

// extractWikilinks returns every `[[slug]]` target found in body, in
// document order. Duplicates are preserved — the caller decides whether
// to dedupe (the similarity layer wants raw counts).
func extractWikilinks(body string) []string {
	matches := wikilinkRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// auditBrokenLinks returns the subset of wikilink targets that resolve to a
// file under one of the deprecatedRoots. The check is filesystem-backed:
// we look for `<root>/<target>.md` and, as a secondary convention,
// `<root>/<target>/SKILL.md` (used by the .claude/skills/_deprecated/
// layout where each retired skill lives in its own directory).
func auditBrokenLinks(targets, deprecatedRoots []string) []string {
	if len(targets) == 0 || len(deprecatedRoots) == 0 {
		return nil
	}
	var broken []string
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if _, dup := seen[target]; dup {
			continue
		}
		seen[target] = struct{}{}
		for _, root := range deprecatedRoots {
			if fileExists(filepath.Join(root, target+".md")) ||
				fileExists(filepath.Join(root, target, "SKILL.md")) {
				broken = append(broken, target)
				break
			}
		}
	}
	return broken
}

// fileExists reports whether path resolves to a regular file. fs.ErrNotExist
// is treated as a clean "no"; any other error (e.g. permission denied) is
// treated as "no" as well — the broken-link audit is best-effort and must
// never derail ingestion.
func fileExists(path string) bool {
	info, statErr := os.Stat(path)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return false
		}
		return false
	}
	return !info.IsDir()
}
