// Memory entry parser. Reads `memory/*.md` files (Claude-style memory entries
// with YAML frontmatter and wikilink references) into MemoryRecords consumable
// by the learning-loop ingest pipeline.
//
// REQ-2 (memory source): every non-index `.md` file under a memory root is one
// memoryRecord. The Tokens slice carries the entry name, its declared type,
// and the targets of every `[[wikilink]]` so downstream similarity (REQ-3) can
// cluster related entries even when their bodies diverge stylistically.
//
// REQ-19a (broken-link audit): when a wikilink target resolves under one of
// the supplied `deprecatedRoots` (e.g. `memory/_deprecated/`,
// `.claude/skills/_deprecated/<name>/SKILL.md`) the target is recorded in
// BrokenLinks. Callers surface these in audit reports.

package learn

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
)

// memoryEntryKind is the constant memoryRecord.Kind value for memory entries.
// It exists so downstream consumers can switch on a typed constant instead of
// a free-form string literal.
const memoryEntryKind = "memory-entry"

// indexFile is the well-known memory index file name. parseMemoryDir skips it
// because it is a hand-curated table of contents, not a memory entry.
const indexFile = "MEMORY.md"

// deprecatedDirName is the sentinel directory under a memory root that
// contains retired entries. parseMemoryDir refuses to ingest its contents.
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

// memoryRecord is one parsed memory entry. It is the contract shared with the
// store layer; field order, types, and zero-value semantics are locked.
type memoryRecord struct {
	// Source is the path of the file as it should appear in store rows.
	// Callers may pass an absolute path to parseMemoryFile; the field reflects
	// whatever was supplied verbatim — normalisation is the caller's job.
	Source string
	// Kind is always memoryEntryKind ("memory-entry").
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
	Frontmatter memoryFrontmatter
	// BrokenLinks lists wikilink targets that resolve under one of the
	// deprecated roots passed to parseMemoryFile (REQ-19a). Empty when no
	// broken link is found or when no deprecatedRoots are supplied.
	BrokenLinks []string
}

// memoryFrontmatter is the structured YAML header at the top of every memory
// entry. Unknown fields are ignored (yaml.v3's default behavior) so the
// schema can grow without breaking the parser.
type memoryFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Metadata    struct {
		Type string `yaml:"type"`
	} `yaml:"metadata"`
}

// parseMemoryFile reads one memory markdown file, parses its YAML frontmatter
// and body, applies the sanitizer to Body and Tokens, and (if deprecatedRoots
// are supplied) audits every wikilink target against them.
//
// Errors are typed for the learnerr exit-code contract (REQ-22): file IO
// failures surface as *runtimeError; format violations (missing frontmatter,
// malformed YAML) surface as *usageError with the failing line cited when
// the YAML library exposes one.
//
// ctx is honored for cancellation between sub-steps but the IO is otherwise
// small enough that fine-grained cancellation would add more complexity than
// value.
func parseMemoryFile(ctx context.Context, path string, deprecatedRoots []string, san *sanitizer) (*memoryRecord, error) {
	if san == nil {
		return nil, usagef("parseMemoryFile: sanitizer is nil (REQ-4 violation)")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, runtimef("parseMemoryFile: %w", ctxErr)
	}

	raw, readErr := os.ReadFile(path) //nolint:gosec // caller-supplied memory path; package purpose is to read it
	if readErr != nil {
		return nil, runtimef("parseMemoryFile: read %q: %w", path, readErr)
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		return nil, runtimef("parseMemoryFile: stat %q: %w", path, statErr)
	}

	fm, body, splitErr := extractMemoryFrontmatter(raw)
	if splitErr != nil {
		return nil, usagef("parseMemoryFile: %s: %w", path, splitErr)
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

	// sanitize every Token (defensive — a leaked secret could in principle
	// reach a wikilink target slug, even if the regex makes that unlikely).
	for i, tok := range tokens {
		tokens[i] = san.sanitize(tok)
	}

	brokenLinks := auditBrokenLinks(wikilinks, deprecatedRoots)

	return &memoryRecord{
		Source:      path,
		Kind:        memoryEntryKind,
		Tokens:      tokens,
		SeenAt:      info.ModTime(),
		Body:        san.sanitize(strings.TrimPrefix(body, "\n")),
		Frontmatter: fm,
		BrokenLinks: brokenLinks,
	}, nil
}

// parseMemoryDir walks `<dir>/*.md` (non-recursive — memory is a flat
// namespace), skipping `MEMORY.md` and anything under `_deprecated/`.
// Per-file errors are logged via log/slog and the walk continues; the only
// error returned is a directory-listing failure.
func parseMemoryDir(ctx context.Context, dir string, deprecatedRoots []string, san *sanitizer) ([]memoryRecord, error) {
	if san == nil {
		return nil, usagef("parseMemoryDir: sanitizer is nil (REQ-4 violation)")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, runtimef("parseMemoryDir: %w", ctxErr)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return nil, runtimef("parseMemoryDir: readdir %q: %w", dir, readErr)
	}

	records := make([]memoryRecord, 0, len(entries))
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
		if isMemoryDeprecated(filepath.Join(dir, name)) {
			continue
		}

		path := filepath.Join(dir, name)
		rec, parseErr := parseMemoryFile(ctx, path, deprecatedRoots, san)
		if parseErr != nil {
			slog.WarnContext(ctx, "parseMemoryDir: skipping file",
				"path", path,
				"error", parseErr.Error(),
			)
			continue
		}
		records = append(records, *rec)
	}
	return records, nil
}

// isMemoryDeprecated reports whether path lives under a `_deprecated/`
// segment. It is a pure path check — no IO — so callers can use it in walks.
func isMemoryDeprecated(path string) bool {
	cleaned := filepath.ToSlash(filepath.Clean(path))
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == deprecatedDirName {
			return true
		}
	}
	return false
}

// extractMemoryFrontmatter splits raw into (parsed frontmatter, body). The
// expected layout is `---\n<yaml>\n---\n<body>`. Any deviation surfaces as
// an error suitable for wrapping in *usageError. The returned body keeps
// its trailing newlines; the leading newline after the closing delimiter is
// trimmed by the caller.
func extractMemoryFrontmatter(raw []byte) (memoryFrontmatter, string, error) {
	text := string(raw)
	if !strings.HasPrefix(text, frontmatterDelim) {
		return memoryFrontmatter{}, "", errors.New("missing frontmatter: file does not start with '---'")
	}

	// Find the closing delimiter. It must appear on its own line; we anchor
	// on "\n---" to avoid matching the opening delimiter or in-body content.
	rest := text[len(frontmatterDelim):]
	// rest must start with a newline so that the YAML body begins on a fresh
	// line. A file that begins with "---<eof>" is malformed.
	if !strings.HasPrefix(rest, "\n") {
		return memoryFrontmatter{}, "", errors.New("missing frontmatter: opening delimiter not followed by newline")
	}
	rest = rest[1:]

	closeIdx := strings.Index(rest, "\n"+frontmatterDelim)
	if closeIdx < 0 {
		return memoryFrontmatter{}, "", errors.New("missing frontmatter: closing '---' delimiter not found")
	}
	yamlBlock := rest[:closeIdx]
	body := rest[closeIdx+len("\n"+frontmatterDelim):]
	// Trim the newline right after the closing delimiter, if present, so the
	// caller does not see a leading blank line.
	body = strings.TrimPrefix(body, "\n")

	var fm memoryFrontmatter
	if unmarshalErr := yaml.Unmarshal([]byte(yamlBlock), &fm); unmarshalErr != nil {
		// yaml.v3 errors typically embed "line N:"; surface them verbatim so
		// the operator can jump to the offending line.
		return memoryFrontmatter{}, "", fmt.Errorf("invalid YAML frontmatter (line/column from yaml): %w", unmarshalErr)
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
			if memoryFileExists(filepath.Join(root, target+".md")) ||
				memoryFileExists(filepath.Join(root, target, "SKILL.md")) {
				broken = append(broken, target)
				break
			}
		}
	}
	return broken
}

// memoryFileExists reports whether path resolves to a regular file.
// fs.ErrNotExist is treated as a clean "no"; any other error (e.g. permission
// denied) is treated as "no" as well — the broken-link audit is best-effort
// and must never derail ingestion.
func memoryFileExists(path string) bool {
	info, statErr := os.Stat(path)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return false
		}
		return false
	}
	return !info.IsDir()
}
