package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSpec_IgnoresHTMLComments(t *testing.T) {
	t.Parallel()
	src := "# Spec: x\n\n## Status: DRAFT\n\n## Slug: ABC\n\n" +
		"<!--\nnoise that must NOT be parsed:\n## Status: WIP\n## Bogus Section\n" +
		"- [ ] ABC-REQ-99: fake\n| ABC-TC-UC-99 | ABC-REQ-99 | happy | x | y |\nBatch 9: [TASK-99]\n-->\n\n" +
		"## Context\n\nreal\n"
	m, err := parseSpec(src)
	require.NoError(t, err)
	assert.Equal(t, "DRAFT", m.status, "real status only; commented WIP ignored")
	assert.Equal(t, 1, m.statusLineCount, "commented ## Status must not count")
	assert.Empty(t, m.reqs, "commented REQ must not be parsed")
	assert.Empty(t, m.tcs, "commented TC must not be parsed")
	assert.Empty(t, m.batches, "commented Batch must not be parsed")
	assert.False(t, m.sections["Bogus Section"], "commented heading must not be a section")

	// An inline trailing comment keeps the real content on the line.
	m2, err2 := parseSpec("## Status: DONE <!-- shipped -->\n")
	require.NoError(t, err2)
	assert.Equal(t, "DONE", m2.status, "inline trailing comment keeps the status token")
}

func TestParseSpec_StatusAndSlug(t *testing.T) {
	t.Parallel()
	content := `## Slug: GOLD

## Status: APPROVED

## Context

Some context.
`
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	assert.Equal(t, "APPROVED", m.status)
	assert.Equal(t, "GOLD", m.slug)
	assert.Equal(t, 1, m.statusLineCount)
	assert.Equal(t, 1, m.slugLineCount)
}

func TestParseSpec_DuplicateStatus(t *testing.T) {
	t.Parallel()
	content := `## Status: DRAFT

## Status: APPROVED
`
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	assert.Equal(t, 2, m.statusLineCount)
	// Only first status is stored
	assert.Equal(t, "DRAFT", m.status)
}

func TestParseSpec_DuplicateSlug(t *testing.T) {
	t.Parallel()
	content := `## Slug: SDDX

## Slug: GOLD
`
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	assert.Equal(t, 2, m.slugLineCount)
	// Only first slug stored
	assert.Equal(t, "SDDX", m.slug)
}

func TestParseSpec_Requirements(t *testing.T) {
	t.Parallel()
	content := `## Requirements

- [ ] REQ-1: First
- [x] REQ-2: Second (no-test: doc-only)
- [ ] REQ-3: Third (no-test:)
`
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	require.Len(t, m.reqs, 3)

	assert.Equal(t, "REQ-1", m.reqs[0].id)
	assert.False(t, m.reqs[0].noTestPresent)

	assert.Equal(t, "REQ-2", m.reqs[1].id)
	assert.True(t, m.reqs[1].noTestPresent)
	assert.Equal(t, "doc-only", m.reqs[1].noTestReason)

	assert.Equal(t, "REQ-3", m.reqs[2].id)
	assert.True(t, m.reqs[2].noTestPresent)
	assert.Equal(t, "", m.reqs[2].noTestReason)
}

func TestParseSpec_TCRows(t *testing.T) {
	t.Parallel()
	content := `## Test Plan

| TC-ID | REQ | Category | Description | Expected |
| --- | --- | --- | --- | --- |
| TC-UC-01 | REQ-1 | happy | does thing | ok |
| TC-D-02 | REQ-2 | edge | another | fail |
`
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	require.Len(t, m.tcs, 2)
	assert.Equal(t, "TC-UC-01", m.tcs[0].id)
	assert.Equal(t, "REQ-1", m.tcs[0].reqRef)
	assert.Equal(t, "TC-D-02", m.tcs[1].id)
}

func TestParseSpec_TCRowIgnoresHeaderAndBadIDs(t *testing.T) {
	t.Parallel()
	// Description column contains a bad ID in backticks — should NOT be parsed as a TC
	content := `## Test Plan

| TC-ID | REQ | Category | Description | Expected |
| --- | --- | --- | --- | --- |
| TC-UC-01 | REQ-1 | happy | uses ` + "`WRONG-TC-UC-99`" + ` internally | ok |
`
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	// Only TC-UC-01 should be parsed, not WRONG-TC-UC-99
	require.Len(t, m.tcs, 1)
	assert.Equal(t, "TC-UC-01", m.tcs[0].id)
}

func TestParseSpec_Tasks(t *testing.T) {
	t.Parallel()
	content := `## Tasks

- [ ] TASK-1: Do something
  - files: internal/domain/x/entity.go
  - tests: TC-UC-01, TC-UC-02
  - depends: TASK-2

- [ ] TASK-2: Do other thing
  - files: internal/domain/x/other.go
`
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	require.Len(t, m.tasks, 2)

	assert.Equal(t, "TASK-1", m.tasks[0].id)
	assert.Equal(t, []string{"internal/domain/x/entity.go"}, m.tasks[0].files)
	assert.Equal(t, []string{"TC-UC-01", "TC-UC-02"}, m.tasks[0].tests)
	assert.Equal(t, []string{"TASK-2"}, m.tasks[0].depends)

	assert.Equal(t, "TASK-2", m.tasks[1].id)
}

func TestParseSpec_Batches(t *testing.T) {
	t.Parallel()
	content := `## Parallel Batches

Batch 1: [TASK-1]

Batch 2: [TASK-2, TASK-3]
- shared-additive: internal/domain/x/shared.go
`
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	require.Len(t, m.batches, 2)

	assert.Equal(t, 1, m.batches[0].index)
	assert.Equal(t, []string{"TASK-1"}, m.batches[0].taskIDs)

	assert.Equal(t, 2, m.batches[1].index)
	assert.Equal(t, []string{"TASK-2", "TASK-3"}, m.batches[1].taskIDs)
	assert.Equal(t, []string{"internal/domain/x/shared.go"}, m.batches[1].sharedAdditive)
}

func TestParseSpec_FencedBlocksIgnored(t *testing.T) {
	t.Parallel()
	content := "## Status: DRAFT\n\n## Context\n\nSome text.\n\n```go\n## Status: WIP\n```\n\n~~~\n## Status: WIP\n~~~\n"
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	// Only the real Status line should be counted
	assert.Equal(t, 1, m.statusLineCount)
	assert.Equal(t, "DRAFT", m.status)
}

func TestParseSpec_BatchLinesAreFenceAgnostic(t *testing.T) {
	t.Parallel()
	// The Parallel Batches section is fenced (```text) in real specs, so batch lines MUST be
	// parsed regardless of fence state — both the outside- and inside-fence batches are captured.
	content := "## Status: DRAFT\n\n## Parallel Batches\n\nBatch 1: [TASK-1]\n\n```\nBatch 2: [TASK-FAKE]\n```\n\n"
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	require.Len(t, m.batches, 2, "batch lines are parsed regardless of fence state")
	assert.Equal(t, 1, m.batches[0].index)
	assert.Equal(t, 2, m.batches[1].index, "the inside-fence batch must also be parsed")
}

func TestParseSpec_NoStatus(t *testing.T) {
	t.Parallel()
	content := `## Context

No status here.
`
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	assert.Equal(t, 0, m.statusLineCount)
	assert.Equal(t, "", m.status)
}

// TC-22 parser: task with `- files: (none — execution only)` → execOnly=true, no files
func TestParseSpec_ExecOnly(t *testing.T) {
	t.Parallel()
	content := "## Tasks\n\n" +
		"- [ ] TASK-SMOKE: run smoke\n" +
		"  - files: (none — execution only)\n" +
		"  - tests: TC-S-01\n"
	m := mustParse(t, content)
	require.Len(t, m.tasks, 1)
	assert.True(t, m.tasks[0].execOnly, "execOnly must be true")
	assert.Empty(t, m.tasks[0].files, "execOnly task must have no files")
	assert.Equal(t, []string{"TC-S-01"}, m.tasks[0].tests)
}

// TC-22 parser: `- files: (none)` sentinel also sets execOnly
func TestParseSpec_ExecOnly_ShortSentinel(t *testing.T) {
	t.Parallel()
	content := "## Tasks\n\n- [ ] TASK-1: impl\n  - files: (none)\n"
	m := mustParse(t, content)
	require.Len(t, m.tasks, 1)
	assert.True(t, m.tasks[0].execOnly)
	assert.Empty(t, m.tasks[0].files)
}

// Named task regex: TASK-SMOKE, TASK-MERGE-SERVER, TASK-FINAL, TASK-12
func TestParseSpec_NamedTaskRegex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		line    string
		wantID  string
		wantHit bool
	}{
		{"TASK-1", "- [ ] TASK-1: impl", "TASK-1", true},
		{"TASK-12", "- [ ] TASK-12: impl", "TASK-12", true},
		{"TASK-SMOKE", "- [ ] TASK-SMOKE: run smoke", "TASK-SMOKE", true},
		{"TASK-MERGE-SERVER", "- [ ] TASK-MERGE-SERVER: merge", "TASK-MERGE-SERVER", true},
		{"TASK-FINAL", "- [ ] TASK-FINAL: finalize", "TASK-FINAL", true},
		{"prose mention", "This depends on TASK-SMOKE for the smoke run", "", false},
		{"no colon-space", "- [ ] TASK-1 no colon", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := reTaskDecl.FindStringSubmatch(tc.line)
			if tc.wantHit {
				require.NotNil(t, m, "should match: %q", tc.line)
				assert.Equal(t, tc.wantID, m[1])
			} else {
				assert.Nil(t, m, "should NOT match: %q", tc.line)
			}
		})
	}
}

func TestParseSpec_SectionsTracked(t *testing.T) {
	t.Parallel()
	content := `## Context

x

## Requirements

- [ ] REQ-1: r

## Design

d
`
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	assert.True(t, m.sections["Context"])
	assert.True(t, m.sections["Requirements"])
	assert.True(t, m.sections["Design"])
	assert.False(t, m.sections["Tasks"])
}
