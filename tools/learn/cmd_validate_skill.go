// validate-skill subcommand (REQ-7). Verifies that a SKILL.md (or other
// learning-loop markdown file) carries the five frontmatter fields required
// for provenance tracking: name, description, learning_provenance,
// created_at, last_reviewed_at. Each missing field is reported as a
// UsageError so the dispatcher exits with code 1.
package learn

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// requiredFrontmatterFields are the five keys every learning-loop authored
// SKILL.md/memory.md must declare. The order matters for the error message
// produced when more than one is missing — operators read top-down.
var requiredFrontmatterFields = []string{
	"name",
	"description",
	"learning_provenance",
	"created_at",
	"last_reviewed_at",
}

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newValidateSkillCmd())
	})
}

// newValidateSkillCmd builds the `learn validate-skill <path>` cobra command.
// No flags: validation is a pure-file operation; the DB is not consulted.
func newValidateSkillCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "validate-skill <path>",
		Short: "Validate that a SKILL.md has the required frontmatter fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidateSkill(validateSkillOpts{
				Path: args[0],
			})
		},
	}
	return c
}

// validateSkillOpts is the typed input to runValidateSkill.
type validateSkillOpts struct {
	Path string
}

// runValidateSkill parses the frontmatter of the file at Path and verifies
// the five required fields are present and non-empty. A missing or empty
// field surfaces as a UsageError citing the offending key.
func runValidateSkill(o validateSkillOpts) error {
	if o.Path == "" {
		return usagef("validate-skill: path is required")
	}
	raw, readErr := os.ReadFile(o.Path)
	if readErr != nil {
		return runtimef("validate-skill: read %q: %w", o.Path, readErr)
	}
	fmMap, parseErr := parseFrontmatterOnly(raw)
	if parseErr != nil {
		return usagef("validate-skill: %s: %w", o.Path, parseErr)
	}

	var missing []string
	for _, field := range requiredFrontmatterFields {
		v, ok := fmMap[field]
		if !ok {
			missing = append(missing, field)
			continue
		}
		if isEmptyFrontmatterValue(v) {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return usagef("validate-skill: %s: missing required frontmatter field(s): %s", o.Path, strings.Join(missing, ", "))
	}
	return nil
}

// parseFrontmatterOnly extracts the YAML block between the leading `---`
// delimiters and returns it as an untyped map. Files without a frontmatter
// block surface as an error suitable for wrapping in UsageError.
func parseFrontmatterOnly(raw []byte) (map[string]any, error) {
	text := string(raw)
	if !strings.HasPrefix(text, "---") {
		return nil, errors.New("missing frontmatter: file does not start with '---'")
	}
	rest := text[3:]
	if !strings.HasPrefix(rest, "\n") {
		return nil, errors.New("missing frontmatter: opening delimiter not followed by newline")
	}
	rest = rest[1:]
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		return nil, errors.New("missing frontmatter: closing '---' not found")
	}
	yamlBlock := rest[:closeIdx]
	m := make(map[string]any)
	if unmarshalErr := yaml.Unmarshal([]byte(yamlBlock), &m); unmarshalErr != nil {
		return nil, fmt.Errorf("invalid YAML frontmatter: %w", unmarshalErr)
	}
	return m, nil
}

// isEmptyFrontmatterValue reports whether a parsed YAML value should be
// treated as "missing" for validation purposes. Empty strings, nil, empty
// lists, and empty maps all count as missing — the operator's intent in
// every case is the same: "this field has no usable content".
func isEmptyFrontmatterValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return false
	}
}
