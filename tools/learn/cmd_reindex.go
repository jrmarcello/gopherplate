// reindex subcommand implementation. Walks the skills directory
// (.claude/skills/<slug>/SKILL.md) and the memory directory (memory/*.md),
// upserts each file into skill_index / memory_index, and removes orphans
// whose source file no longer exists on disk (REQ-19, REQ-19a, REQ-20).
//
// Files under any `_deprecated/` path segment are excluded from indexing.
// An orphan row is preserved when the source file moved to a deprecated
// counterpart (`<dir>/_deprecated/<basename>.md` or
// `<dir>/_deprecated/<slug>/SKILL.md`), so retired-skill rows survive the
// move and stay surfaceable by the broken-link audit (REQ-19a).
package learn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	defaultSkillsDir = ".claude/skills"
	defaultMemoryDir = "memory"
	skillFileName    = "SKILL.md"
	deprecatedDir    = "_deprecated"
)

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newReindexCmd())
	})
}

// newReindexCmd builds the `learn reindex` cobra command.
func newReindexCmd() *cobra.Command {
	var (
		dbPath     string
		skillsDir  string
		memoryDir  string
		singlePath string
		sinceMtime time.Duration
		configPath string
	)

	c := &cobra.Command{
		Use:   "reindex",
		Short: "Reindex skills and memory into the learning store",
		Long: `reindex walks the configured skills directory (default .claude/skills) and the
memory directory (default memory/), parses each entry's frontmatter and body,
and upserts the resulting rows into the SQLite store. Re-running reindex on an
unchanged tree is a no-op; rows whose source file has been deleted are pruned
(unless a deprecated counterpart exists).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReindex(cmd.Context(), reindexOpts{
				DBPath:     dbPath,
				SkillsDir:  skillsDir,
				MemoryDir:  memoryDir,
				SinglePath: singlePath,
				SinceMtime: sinceMtime,
				ConfigPath: configPath,
				Stderr:     cmd.ErrOrStderr(),
			})
		},
	}
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite", "Path to the SQLite store")
	c.Flags().StringVar(&skillsDir, "skills-dir", defaultSkillsDir, "Directory containing skills (<slug>/SKILL.md)")
	c.Flags().StringVar(&memoryDir, "memory-dir", defaultMemoryDir, "Directory containing memory entries (*.md, non-recursive)")
	c.Flags().StringVar(&singlePath, "path", "", "Reindex only this single file (incremental mode)")
	c.Flags().DurationVar(&sinceMtime, "since-mtime", 0, "Only reindex files whose mtime is within this duration of now")
	c.Flags().StringVar(&configPath, "config", ".claude/learning/config.yml", "Path to learning config (for sanitize patterns)")
	return c
}

// reindexOpts is the input to runReindex. All fields have sane defaults
// applied internally; tests inject overrides for determinism.
type reindexOpts struct {
	DBPath     string
	SkillsDir  string
	MemoryDir  string
	SinglePath string
	SinceMtime time.Duration
	ConfigPath string
	Stderr     io.Writer
	Now        func() time.Time
}

// runReindex is the entry point exercised by tests. It applies defaults,
// opens the store, builds a sanitizer, walks the configured directories,
// upserts rows, and prunes orphans.
func runReindex(ctx context.Context, o reindexOpts) error {
	if o.DBPath == "" {
		return usagef("reindex: --db-path must be non-empty")
	}
	if o.Now == nil {
		o.Now = func() time.Time { return time.Now().UTC() }
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}

	cfg, loadErr := loadConfig(o.ConfigPath)
	if loadErr != nil {
		return runtimef("reindex: load config: %w", loadErr)
	}
	san, sanErr := newSanitizerFromConfig(cfg.SecretPatterns)
	if sanErr != nil {
		return runtimef("reindex: build sanitizer: %w", sanErr)
	}

	st, openErr := openStore(o.DBPath)
	if openErr != nil {
		return runtimef("reindex: open store: %w", openErr)
	}
	defer func() { _ = st.Close() }()

	// Single-path mode: only update one entry, no orphan sweep.
	if o.SinglePath != "" {
		return reindexSinglePath(ctx, st, san, o)
	}

	now := o.Now()

	// Collect on-disk paths so we can compute orphans after upserts.
	skillsOnDisk, skillScanErr := scanSkillFiles(o.SkillsDir, o.SinceMtime, now)
	if skillScanErr != nil {
		return skillScanErr
	}
	memoryOnDisk, memoryScanErr := scanMemoryFiles(o.MemoryDir, o.SinceMtime, now)
	if memoryScanErr != nil {
		return memoryScanErr
	}

	for _, path := range skillsOnDisk {
		if upsertErr := upsertSkill(ctx, st, san, path, now); upsertErr != nil {
			return upsertErr
		}
	}

	deprecatedRoots := computeDeprecatedRoots(o)
	for _, path := range memoryOnDisk {
		if upsertErr := upsertMemory(ctx, st, san, path, deprecatedRoots, now); upsertErr != nil {
			return upsertErr
		}
	}

	if pruneErr := pruneOrphanSkills(ctx, st, o.SkillsDir); pruneErr != nil {
		return pruneErr
	}
	if pruneErr := pruneOrphanMemory(ctx, st, o.MemoryDir); pruneErr != nil {
		return pruneErr
	}

	return nil
}

// reindexSinglePath handles the `--path <file>` incremental mode.
func reindexSinglePath(ctx context.Context, st *sqliteStore, san *sanitizer, o reindexOpts) error {
	info, statErr := os.Stat(o.SinglePath)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return usagef("reindex: --path %q: file does not exist", o.SinglePath)
		}
		return runtimef("reindex: stat %q: %w", o.SinglePath, statErr)
	}
	if info.IsDir() {
		return usagef("reindex: --path %q: is a directory, expected a file", o.SinglePath)
	}
	if isDeprecatedPath(o.SinglePath) {
		// Silently skip — deprecated files are not indexed.
		return nil
	}

	now := o.Now()
	base := filepath.Base(o.SinglePath)
	switch {
	case base == skillFileName:
		return upsertSkill(ctx, st, san, o.SinglePath, now)
	case strings.HasSuffix(base, ".md"):
		deprecatedRoots := computeDeprecatedRoots(o)
		return upsertMemory(ctx, st, san, o.SinglePath, deprecatedRoots, now)
	default:
		return usagef("reindex: --path %q: unsupported file (expected SKILL.md or *.md)", o.SinglePath)
	}
}

// scanSkillFiles walks <skillsDir>/**/SKILL.md, excluding any path under a
// `_deprecated/` segment. Returns paths sorted for determinism.
func scanSkillFiles(skillsDir string, sinceMtime time.Duration, now time.Time) ([]string, error) {
	if _, statErr := os.Stat(skillsDir); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, runtimef("reindex: stat skills dir %q: %w", skillsDir, statErr)
	}

	var out []string
	walkErr := filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == deprecatedDir {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != skillFileName {
			return nil
		}
		if isDeprecatedPath(path) {
			return nil
		}
		if !mtimeMatches(path, sinceMtime, now) {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if walkErr != nil {
		return nil, runtimef("reindex: walk skills %q: %w", skillsDir, walkErr)
	}
	sort.Strings(out)
	return out, nil
}

// scanMemoryFiles enumerates <memoryDir>/*.md non-recursively, excluding
// MEMORY.md and any file under `_deprecated/`. Returns paths sorted for
// determinism.
func scanMemoryFiles(memoryDir string, sinceMtime time.Duration, now time.Time) ([]string, error) {
	entries, readErr := os.ReadDir(memoryDir)
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, runtimef("reindex: readdir memory %q: %w", memoryDir, readErr)
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "MEMORY.md" {
			continue
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		path := filepath.Join(memoryDir, name)
		if isDeprecatedPath(path) {
			continue
		}
		if !mtimeMatches(path, sinceMtime, now) {
			continue
		}
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

// mtimeMatches reports whether the given path's mtime is within the supplied
// duration of `now`. A zero `since` means "no filter" — always include.
func mtimeMatches(path string, since time.Duration, now time.Time) bool {
	if since == 0 {
		return true
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		return false
	}
	return now.Sub(info.ModTime()) <= since
}

// isDeprecatedPath reports whether path contains a `_deprecated` directory
// segment. Pure path check — no IO.
func isDeprecatedPath(path string) bool {
	cleaned := filepath.ToSlash(filepath.Clean(path))
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == deprecatedDir {
			return true
		}
	}
	return false
}

// computeDeprecatedRoots returns the directories the memory parser should
// audit wikilinks against to flag broken links (REQ-19a).
func computeDeprecatedRoots(o reindexOpts) []string {
	roots := []string{}
	if o.MemoryDir != "" {
		roots = append(roots, filepath.Join(o.MemoryDir, deprecatedDir))
	}
	if o.SkillsDir != "" {
		roots = append(roots, filepath.Join(o.SkillsDir, deprecatedDir))
	}
	return roots
}

// upsertSkill parses one SKILL.md file and upserts it into skill_index.
func upsertSkill(ctx context.Context, st *sqliteStore, san *sanitizer, path string, now time.Time) error {
	parsed, parseErr := parseSkillFile(path, san)
	if parseErr != nil {
		return parseErr
	}
	entry := skillIndexEntry{
		Path:            path,
		Title:           parsed.Title,
		Body:            parsed.Body,
		Tags:            parsed.Tags,
		FrontmatterJSON: parsed.FrontmatterJSON,
		IndexedAt:       now.Format(time.RFC3339Nano),
	}
	if upsertErr := st.UpsertSkillIndex(ctx, entry); upsertErr != nil {
		return runtimef("reindex: upsert skill %q: %w", path, upsertErr)
	}
	return nil
}

// upsertMemory parses one memory markdown file and upserts it into
// memory_index. Broken wikilinks are surfaced as slog warns (REQ-19a).
func upsertMemory(ctx context.Context, st *sqliteStore, san *sanitizer, path string, deprecatedRoots []string, now time.Time) error {
	rec, parseErr := parseMemoryFile(ctx, path, deprecatedRoots, san)
	if parseErr != nil {
		return runtimef("reindex: parse memory %q: %w", path, parseErr)
	}

	for _, target := range rec.BrokenLinks {
		slog.WarnContext(ctx, "broken_link",
			"source", path,
			"target_deprecated", target,
		)
	}

	fmJSON, marshalErr := json.Marshal(rec.Frontmatter)
	if marshalErr != nil {
		return runtimef("reindex: marshal frontmatter %q: %w", path, marshalErr)
	}
	title := rec.Frontmatter.Name
	if title == "" {
		title = filepath.Base(path)
	}
	entry := memoryIndexEntry{
		Path:            path,
		Title:           title,
		Body:            rec.Body,
		FrontmatterJSON: string(fmJSON),
		IndexedAt:       now.Format(time.RFC3339Nano),
	}
	if upsertErr := st.UpsertMemoryIndex(ctx, entry); upsertErr != nil {
		return runtimef("reindex: upsert memory %q: %w", path, upsertErr)
	}
	return nil
}

// parsedSkill is the structured view of a SKILL.md file produced by the
// inline frontmatter+body extractor. It is private to reindex.go because
// it is a single-call-site helper.
type parsedSkill struct {
	Path            string
	Title           string
	Body            string
	Tags            string
	FrontmatterJSON string
}

// skillFrontmatter is the YAML header shape we accept for skills. Unknown
// keys are ignored; the raw frontmatter is also marshaled to JSON for
// archival in frontmatter_json.
type skillFrontmatter struct {
	Name        string   `yaml:"name" json:"name,omitempty"`
	Description string   `yaml:"description" json:"description,omitempty"`
	Tags        []string `yaml:"tags" json:"tags,omitempty"`
	// ArgumentHint and other free-form fields survive via the raw map below.
	ArgumentHint string `yaml:"argument-hint" json:"argument-hint,omitempty"`
}

// parseSkillFile reads a SKILL.md, extracts its YAML frontmatter and body,
// applies the sanitizer, and produces a parsedSkill ready for upsert.
func parseSkillFile(path string, san *sanitizer) (*parsedSkill, error) {
	if san == nil {
		return nil, usagef("reindex: parseSkillFile: sanitizer is nil")
	}
	raw, readErr := os.ReadFile(path) //nolint:gosec // path comes from filepath.WalkDir inside the operator-supplied --skills-dir
	if readErr != nil {
		return nil, runtimef("reindex: read %q: %w", path, readErr)
	}

	fm, body, splitErr := extractSkillFrontmatter(raw)
	if splitErr != nil {
		return nil, usagef("reindex: parse %q: %w", path, splitErr)
	}

	title := fm.Name
	if title == "" {
		// Fall back to first H1 in the body.
		title = firstH1(body)
	}
	if title == "" {
		title = filepath.Base(filepath.Dir(path))
	}

	tags := strings.Join(fm.Tags, ",")

	fmJSON, marshalErr := json.Marshal(fm)
	if marshalErr != nil {
		return nil, runtimef("reindex: marshal frontmatter %q: %w", path, marshalErr)
	}

	return &parsedSkill{
		Path:            path,
		Title:           san.sanitize(title),
		Body:            san.sanitize(strings.TrimPrefix(body, "\n")),
		Tags:            san.sanitize(tags),
		FrontmatterJSON: string(fmJSON),
	}, nil
}

// extractSkillFrontmatter mirrors memory.extractFrontmatter but unmarshals
// into the local skillFrontmatter shape. Returns (fm, body, error).
func extractSkillFrontmatter(raw []byte) (skillFrontmatter, string, error) {
	const delim = "---"
	text := string(raw)
	if !strings.HasPrefix(text, delim) {
		return skillFrontmatter{}, "", errors.New("missing frontmatter: file does not start with '---'")
	}
	rest := text[len(delim):]
	if !strings.HasPrefix(rest, "\n") {
		return skillFrontmatter{}, "", errors.New("missing frontmatter: opening delimiter not followed by newline")
	}
	rest = rest[1:]
	closeIdx := strings.Index(rest, "\n"+delim)
	if closeIdx < 0 {
		return skillFrontmatter{}, "", errors.New("missing frontmatter: closing '---' delimiter not found")
	}
	yamlBlock := rest[:closeIdx]
	body := rest[closeIdx+len("\n"+delim):]
	body = strings.TrimPrefix(body, "\n")

	var fm skillFrontmatter
	if unmarshalErr := yaml.Unmarshal([]byte(yamlBlock), &fm); unmarshalErr != nil {
		return skillFrontmatter{}, "", fmt.Errorf("invalid YAML frontmatter: %w", unmarshalErr)
	}
	return fm, body, nil
}

// firstH1 returns the text of the first `# ` heading in the body, or "" if
// none is found.
func firstH1(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

// pruneOrphanSkills deletes skill_index rows whose path no longer exists on
// disk AND has no deprecated counterpart at either:
//
//	<dir>/_deprecated/<basename>.md
//	<dir>/_deprecated/<slug>/SKILL.md
//
// This preserves rows for skills that moved from an active location into
// `_deprecated/` (TC-UC-15a).
func pruneOrphanSkills(ctx context.Context, st *sqliteStore, skillsDir string) error {
	rows, queryErr := st.DB().QueryContext(ctx, `SELECT path FROM skill_index`)
	if queryErr != nil {
		return runtimef("reindex: list skill_index: %w", queryErr)
	}
	defer func() { _ = rows.Close() }()
	var paths []string
	for rows.Next() {
		var p string
		if scanErr := rows.Scan(&p); scanErr != nil {
			return runtimef("reindex: scan skill_index: %w", scanErr)
		}
		paths = append(paths, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return runtimef("reindex: iterate skill_index: %w", rowsErr)
	}

	for _, p := range paths {
		if fileExists(p) {
			continue
		}
		if deprecatedCounterpartExistsForSkill(p, skillsDir) {
			continue
		}
		if delErr := st.DeleteOrphanSkill(ctx, p); delErr != nil {
			return runtimef("reindex: delete orphan skill %q: %w", p, delErr)
		}
	}
	return nil
}

// pruneOrphanMemory mirrors pruneOrphanSkills for memory_index. The
// deprecated counterpart convention for memory is `<memoryDir>/_deprecated/<basename>`.
func pruneOrphanMemory(ctx context.Context, st *sqliteStore, memoryDir string) error {
	rows, queryErr := st.DB().QueryContext(ctx, `SELECT path FROM memory_index`)
	if queryErr != nil {
		return runtimef("reindex: list memory_index: %w", queryErr)
	}
	defer func() { _ = rows.Close() }()
	var paths []string
	for rows.Next() {
		var p string
		if scanErr := rows.Scan(&p); scanErr != nil {
			return runtimef("reindex: scan memory_index: %w", scanErr)
		}
		paths = append(paths, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return runtimef("reindex: iterate memory_index: %w", rowsErr)
	}

	for _, p := range paths {
		if fileExists(p) {
			continue
		}
		if deprecatedCounterpartExistsForMemory(p, memoryDir) {
			continue
		}
		if delErr := st.DeleteOrphanMemory(ctx, p); delErr != nil {
			return runtimef("reindex: delete orphan memory %q: %w", p, delErr)
		}
	}
	return nil
}

// deprecatedCounterpartExistsForSkill checks whether a deprecated counterpart
// for the original skill path exists. The original path is expected to be
// `<skillsDir>/<slug>/SKILL.md`; the deprecated counterparts checked are
// `<skillsDir>/_deprecated/<slug>.md` and `<skillsDir>/_deprecated/<slug>/SKILL.md`.
func deprecatedCounterpartExistsForSkill(origPath, skillsDir string) bool {
	// Derive slug from the parent directory of SKILL.md.
	parent := filepath.Dir(origPath)
	slug := filepath.Base(parent)
	if slug == "" || slug == "." {
		return false
	}
	candidates := []string{
		filepath.Join(skillsDir, deprecatedDir, slug+".md"),
		filepath.Join(skillsDir, deprecatedDir, slug, skillFileName),
	}
	for _, c := range candidates {
		if fileExists(c) {
			return true
		}
	}
	return false
}

// deprecatedCounterpartExistsForMemory mirrors the skill variant for
// memory entries. Original `<memoryDir>/<name>.md` →
// `<memoryDir>/_deprecated/<name>.md`.
func deprecatedCounterpartExistsForMemory(origPath, memoryDir string) bool {
	base := filepath.Base(origPath)
	return fileExists(filepath.Join(memoryDir, deprecatedDir, base))
}

// fileExists reports whether path refers to a regular file. Symlinks and any
// stat error other than fs.ErrNotExist are treated as "no" — best-effort.
func fileExists(path string) bool {
	info, statErr := os.Stat(path)
	if statErr != nil {
		return false
	}
	return !info.IsDir()
}
