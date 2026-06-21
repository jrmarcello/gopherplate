package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// warnOnlySpec is a structurally-valid spec with exactly one WARN (a task touching a production
// .go file with no tests:) and zero ERRORs — the fixture for the warn-only → exit 0 path.
func warnOnlySpec() string {
	return "## Slug: WO\n" +
		"## Status: DRAFT\n\n" +
		"## Context\n\nsee docs/capabilities/x.md\n\n" +
		"## Requirements\n\n- [ ] WO-REQ-1: r\n\n" +
		"## Test Plan\n\n| TC | REQ | Category | Description | Expected |\n" +
		"| --- | --- | --- | --- | --- |\n| WO-TC-UC-01 | WO-REQ-1 | happy | x | y |\n\n" +
		"## Design\n\nd\n\n" +
		"## Tasks\n\n- [ ] TASK-1: impl\n  - files: internal/domain/x/entity.go\n\n" +
		"## Parallel Batches\n\nBatch 1: [TASK-1]\n\n" +
		"## Validation Criteria\n\nall pass\n\n" +
		"## Execution Log\n\n"
}

// TestRun_WarnOnlySpec_ExitsZero covers SDDX-TC-UC-16's warn-only branch: a spec with WARNs but no
// ERRORs must exit 0 (guards a mutation of the `!hasError → return 0` arm in runLint).
func TestRun_WarnOnlySpec_ExitsZero(t *testing.T) {
	t.Parallel()
	fs := validate(mustParse(t, warnOnlySpec()))
	require.Equal(t, 0, errors(fs), "fixture must have 0 errors")
	require.Equal(t, 1, warns(fs), "fixture must have exactly 1 warn (goProdTaskHasTests)")

	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(path, []byte(warnOnlySpec()), 0o644))
	assert.Equal(t, 0, run([]string{"validate-spec", path}), "warn-only spec must exit 0, not 1")
}

// TestParseSpec_FilesCommaSplit guards the fix that splits a multi-file `- files:` line on commas
// (a single stored string would break excludedByPath and batchFileOverlap on real specs).
func TestParseSpec_FilesCommaSplit(t *testing.T) {
	t.Parallel()
	content := "## Slug: T\n## Status: DRAFT\n\n## Tasks\n\n" +
		"- [ ] TASK-1: x\n  - files: internal/domain/x/a.go, internal/domain/x/b.go\n"
	m := mustParse(t, content)
	require.Len(t, m.tasks, 1)
	assert.Equal(t,
		[]string{"internal/domain/x/a.go", "internal/domain/x/b.go"},
		m.tasks[0].files,
		"a comma-separated `- files:` line must be split into individual paths")
}

// TestValidate_CapabilityLink_CommentOnly_Warns guards the fix that checks the comment-stripped
// body: a docs/capabilities link living ONLY inside an HTML comment must NOT satisfy the check.
func TestValidate_CapabilityLink_CommentOnly_Warns(t *testing.T) {
	t.Parallel()
	content := "## Slug: T\n## Status: DRAFT\n\n" +
		"<!-- TODO: link docs/capabilities/x.md here -->\n\n## Context\n\nno real link\n"
	fs := validate(mustParse(t, content))
	assert.True(t, hasWarn(fs, "docs/capabilities"),
		"a capability link only inside an HTML comment must not suppress the WARN")
}

// TestValidate_DependsDisjointCycles confirms both disjoint cycles are reported (the cycle
// detector must not blanket-mark unvisited nodes and drop a second component's cycle).
func TestValidate_DependsDisjointCycles(t *testing.T) {
	t.Parallel()
	content := "## Slug: T\n## Status: DRAFT\n\n## Tasks\n\n" +
		"- [ ] TASK-1: a\n  - files: a\n  - depends: TASK-2\n" +
		"- [ ] TASK-2: b\n  - files: b\n  - depends: TASK-1\n" +
		"- [ ] TASK-3: c\n  - files: c\n  - depends: TASK-4\n" +
		"- [ ] TASK-4: d\n  - files: d\n  - depends: TASK-3\n"
	fs := validateDependsDAG(mustParse(t, content))
	assert.Equal(t, 2, countFindings(fs, severityError, "cycle"),
		"both disjoint dependency cycles must be reported")
}

// TestParseCapability_GuaranteedReqsScoped guards that only items under
// ## Guaranteed Requirements are captured — not a `-REQ-` mention in prose elsewhere.
func TestParseCapability_GuaranteedReqsScoped(t *testing.T) {
	t.Parallel()
	content := "# Capability: x\n## Slug: X\n## Status: Active\n\n" +
		"## Guarantees (current truth)\n\n- references X-REQ-9 in prose, not a guarantee\n\n" +
		"## Guaranteed Requirements\n\n- X-REQ-1: the real one\n"
	capDoc, ok := parseCapability("x", content)
	require.True(t, ok)
	assert.Equal(t, []string{"X-REQ-1: the real one"}, capDoc.guaranteedReqs,
		"only ## Guaranteed Requirements items are captured, not prose -REQ- mentions")
}

// mustParse parses spec content and fails the test on a (never-expected) parse error.
func mustParse(t *testing.T, content string) model {
	t.Helper()
	m, err := parseSpec(content)
	require.NoError(t, err)
	return m
}
