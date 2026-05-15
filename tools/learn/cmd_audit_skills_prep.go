// audit-skills-prep subcommand (REQ-25). Walks <dir>/**/SKILL.md (excluding
// any _deprecated/ subtree), reads the frontmatter and a leading body
// excerpt of each file, and prints a structured JSON document on stdout.
//
// This is the deterministic helper consumed by the agentic
// /learn-audit-skills skill — it never mutates the skill files (TC-E2E-12a).
package learn

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const defaultMaxBodyChars = 800

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newAuditSkillsPrepCmd())
	})
}

// newAuditSkillsPrepCmd builds the `learn audit-skills-prep` cobra command.
func newAuditSkillsPrepCmd() *cobra.Command {
	var (
		skillsDir    string
		maxBodyChars int
		dbPath       string
	)
	c := &cobra.Command{
		Use:   "audit-skills-prep",
		Short: "Emit a JSON summary of every SKILL.md under a directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuditSkillsPrep(auditSkillsPrepOpts{
				SkillsDir:    skillsDir,
				MaxBodyChars: maxBodyChars,
				DBPath:       dbPath,
				Stdout:       cmd.OutOrStdout(),
			})
		},
	}
	c.Flags().StringVar(&skillsDir, "skills-dir", ".claude/skills", "Directory containing skill subtrees")
	c.Flags().IntVar(&maxBodyChars, "max-body-chars", defaultMaxBodyChars, "Maximum characters of the body excerpt per skill")
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite", "Path to the SQLite store (reserved; unused today)")
	return c
}

// auditSkillsPrepOpts is the typed input to runAuditSkillsPrep.
type auditSkillsPrepOpts struct {
	SkillsDir    string
	MaxBodyChars int
	DBPath       string
	Stdout       io.Writer
}

// skillSummary is one element of the JSON output. Field names are part of
// the wire contract consumed by /learn-audit-skills; renames are breaking.
type skillSummary struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	ArgumentHint  string `json:"argument_hint"`
	UserInvocable bool   `json:"user_invocable"`
	BodyExcerpt   string `json:"body_excerpt"`
	BodySizeBytes int    `json:"body_size_bytes"`
	Tags          string `json:"tags"`
}

// auditSkillsPrepOutput is the top-level JSON document emitted on stdout.
type auditSkillsPrepOutput struct {
	Skills []skillSummary `json:"skills"`
}

// runAuditSkillsPrep walks skillsDir, collects one skillSummary per
// SKILL.md, and writes the document to Stdout. The walk is read-only — no
// timestamps on the discovered files are touched (TC-E2E-12a).
//
// REQ-4 defense-in-depth: even though SKILL.md files should not contain
// secrets, every body excerpt and metadata string is passed through a
// `sanitizer` before being written to stdout. The agent that
// consumes this output may persist it to .specs/reports/ (versioned), so
// any accidental paste of a token or AWS key is redacted at the source.
func runAuditSkillsPrep(o auditSkillsPrepOpts) error {
	if o.SkillsDir == "" {
		return usagef("audit-skills-prep: --skills-dir must be non-empty")
	}
	if o.MaxBodyChars <= 0 {
		o.MaxBodyChars = defaultMaxBodyChars
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}

	san, sanErr := newSanitizerFromConfig(defaultSecretPatterns)
	if sanErr != nil {
		return runtimef("audit-skills-prep: build sanitizer: %w", sanErr)
	}

	var skills []skillSummary
	walkErr := filepath.WalkDir(o.SkillsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "_deprecated" {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "SKILL.md" {
			return nil
		}
		summary, parseErr := summariseSkillFile(path, o.MaxBodyChars, san)
		if parseErr != nil {
			return parseErr
		}
		skills = append(skills, summary)
		return nil
	})
	if walkErr != nil {
		// Distinguish "dir missing" (usage) from other walk errors (runtime).
		// errors.As traverses wrapping so a *fs.PathError nested under
		// fmt.Errorf still surfaces here — the bare type assertion would miss it.
		var pe *fs.PathError
		if errors.As(walkErr, &pe) && os.IsNotExist(pe) {
			return usagef("audit-skills-prep: --skills-dir %q does not exist", o.SkillsDir)
		}
		var le *usageError
		if errors.As(walkErr, &le) {
			return walkErr
		}
		return runtimef("audit-skills-prep: walk %q: %w", o.SkillsDir, walkErr)
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Path < skills[j].Path })

	out := auditSkillsPrepOutput{Skills: skills}
	enc := json.NewEncoder(o.Stdout)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(&out); encErr != nil {
		return runtimef("audit-skills-prep: encode output: %w", encErr)
	}
	return nil
}

// summariseSkillFile reads a SKILL.md and produces its skillSummary. The
// frontmatter is parsed via yaml.v3; the body excerpt is the first
// `maxBodyChars` UTF-8 characters of the body, trimmed of surrounding
// whitespace. The full body size (in bytes) is reported separately so the
// agent can decide whether to read more.
func summariseSkillFile(path string, maxBodyChars int, san *sanitizer) (skillSummary, error) {
	raw, readErr := os.ReadFile(path) //nolint:gosec // path comes from filepath.WalkDir inside the operator-supplied --skills-dir
	if readErr != nil {
		return skillSummary{}, runtimef("audit-skills-prep: read %q: %w", path, readErr)
	}

	fmMap, body, splitErr := splitFrontmatterAndBody(raw)
	if splitErr != nil {
		return skillSummary{}, usagef("audit-skills-prep: %s: %w", path, splitErr)
	}

	trimmedBody := strings.TrimSpace(body)
	excerpt := truncateUTF8(trimmedBody, maxBodyChars)

	// Sanitize every emitted field — REQ-4 defense-in-depth.
	return skillSummary{
		Path:          path,
		Name:          san.sanitize(stringField(fmMap, "name")),
		Description:   san.sanitize(stringField(fmMap, "description")),
		ArgumentHint:  san.sanitize(stringField(fmMap, "argument_hint")),
		UserInvocable: boolField(fmMap, "user_invocable"),
		BodyExcerpt:   san.sanitize(excerpt),
		BodySizeBytes: len(body),
		Tags:          san.sanitize(stringField(fmMap, "tags")),
	}, nil
}

// splitFrontmatterAndBody splits `---\n<yaml>\n---\n<body>` into its parts.
// Missing frontmatter is an error — every SKILL.md in this project is
// expected to carry one.
func splitFrontmatterAndBody(raw []byte) (frontmatter map[string]any, body string, err error) {
	text := string(raw)
	if !strings.HasPrefix(text, "---") {
		return nil, "", fmt.Errorf("missing frontmatter")
	}
	rest := text[3:]
	if !strings.HasPrefix(rest, "\n") {
		return nil, "", fmt.Errorf("missing frontmatter: no newline after opening '---'")
	}
	rest = rest[1:]
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		return nil, "", fmt.Errorf("missing frontmatter: no closing '---'")
	}
	yamlBlock := rest[:closeIdx]
	body = strings.TrimPrefix(rest[closeIdx+len("\n---"):], "\n")
	m := make(map[string]any)
	if unmarshalErr := yaml.Unmarshal([]byte(yamlBlock), &m); unmarshalErr != nil {
		return nil, "", fmt.Errorf("invalid YAML: %w", unmarshalErr)
	}
	return m, body, nil
}

// stringField returns the frontmatter value at key as a string, coercing
// scalar non-strings (bool, int) through fmt.Sprint for resilience against
// authors who forgot to quote.
func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// boolField returns the frontmatter value at key as a bool. Defaults to
// false when the key is absent or the value cannot be coerced — the rubric
// in /learn-audit-skills treats absence as "not user-invocable".
func boolField(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		switch strings.ToLower(s) {
		case "true", "yes", "1":
			return true
		}
	}
	return false
}

// truncateUTF8 returns the first n runes of s. Operates on runes rather than
// bytes so a multi-byte character is never split across the boundary.
func truncateUTF8(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
