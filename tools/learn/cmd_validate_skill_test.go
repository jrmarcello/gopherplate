package learn

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkillFileForValidate materializes a SKILL.md with the supplied frontmatter
// fragment (verbatim; no trailing newline appended). Returns its path.
func writeSkillFileForValidate(t *testing.T, frontmatter string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	content := "---\n" + frontmatter + "---\nbody.\n"
	if writeErr := os.WriteFile(path, []byte(content), 0o644); writeErr != nil {
		t.Fatalf("write skill: %v", writeErr)
	}
	return path
}

// TC-D-08a..e: each individual required field, missing in isolation, surfaces
// as a UsageError naming the field.
func TestRunValidateSkill_missingIndividualFields_returnUsageError(t *testing.T) {
	allFields := map[string]string{
		"name":                "name: foo\n",
		"description":         "description: desc\n",
		"learning_provenance": "learning_provenance:\n  - sig-1\n",
		"created_at":          "created_at: 2026-05-14T00:00:00Z\n",
		"last_reviewed_at":    "last_reviewed_at: 2026-05-14T00:00:00Z\n",
	}

	for missing := range allFields {
		t.Run("missing-"+missing, func(t *testing.T) {
			var frontmatter strings.Builder
			for field, line := range allFields {
				if field == missing {
					continue
				}
				frontmatter.WriteString(line)
			}
			path := writeSkillFileForValidate(t, frontmatter.String())
			runErr := runValidateSkill(validateSkillOpts{Path: path})
			if runErr == nil {
				t.Fatalf("expected UsageError citing %q, got nil", missing)
			}
			var ue *usageError
			if !errors.As(runErr, &ue) {
				t.Fatalf("want *usageError, got %T: %v", runErr, runErr)
			}
			if !strings.Contains(runErr.Error(), missing) {
				t.Errorf("error must cite %q; got: %v", missing, runErr)
			}
		})
	}
}

// TC-D-08f (happy): all 5 fields present → exit 0.
func TestRunValidateSkill_allFieldsPresent_exitZero(t *testing.T) {
	path := writeSkillFileForValidate(t,
		"name: foo\ndescription: bar\nlearning_provenance:\n  - sig-1\ncreated_at: 2026-05-14T00:00:00Z\nlast_reviewed_at: 2026-05-14T00:00:00Z\n",
	)
	if runErr := runValidateSkill(validateSkillOpts{Path: path}); runErr != nil {
		t.Fatalf("want nil error, got %v", runErr)
	}
}

// TC-D-08f (extra): invalid YAML is reported as a UsageError.
func TestRunValidateSkill_invalidYAML_returnsUsageError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	// Tab inside YAML indentation is invalid per the YAML spec; yaml.v3 surfaces
	// it cleanly so we can verify the error path without resorting to other
	// malformations that the parser may quietly accept.
	const bad = "---\nname: foo\n\tbad: \"unbalanced\nlast_reviewed_at: 2026-05-14T00:00:00Z\n---\nbody\n"
	if writeErr := os.WriteFile(path, []byte(bad), 0o644); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	runErr := runValidateSkill(validateSkillOpts{Path: path})
	if runErr == nil {
		t.Fatalf("expected error, got nil")
	}
	var ue *usageError
	if !errors.As(runErr, &ue) {
		t.Errorf("want UsageError, got %T: %v", runErr, runErr)
	}
}

// Edge: empty value (e.g. `name:`) counts as missing.
func TestRunValidateSkill_emptyValueCountsAsMissing(t *testing.T) {
	path := writeSkillFileForValidate(t,
		"name:\ndescription: bar\nlearning_provenance:\n  - sig-1\ncreated_at: 2026-05-14T00:00:00Z\nlast_reviewed_at: 2026-05-14T00:00:00Z\n",
	)
	runErr := runValidateSkill(validateSkillOpts{Path: path})
	if runErr == nil {
		t.Fatalf("expected UsageError for empty 'name', got nil")
	}
	if !strings.Contains(runErr.Error(), "name") {
		t.Errorf("error must cite 'name'; got %v", runErr)
	}
}
