package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadSpec reads a spec fixture from testdata and parses it.
func loadSpec(t *testing.T, paths ...string) model {
	t.Helper()
	parts := append([]string{"testdata"}, paths...)
	path := filepath.Join(parts...)
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr, "reading fixture %s", path)
	m, parseErr := parseSpec(string(data))
	require.NoError(t, parseErr, "parsing fixture %s", path)
	return m
}

// countFindings returns the number of findings with the given severity and optional message substring.
func countFindings(fs []finding, sev severity, msgSubstr string) int {
	n := 0
	for _, f := range fs {
		if f.sev != sev {
			continue
		}
		if msgSubstr == "" || strings.Contains(f.msg, msgSubstr) {
			n++
		}
	}
	return n
}

func hasError(fs []finding, substr string) bool {
	return countFindings(fs, severityError, substr) > 0
}

func hasWarn(fs []finding, substr string) bool {
	return countFindings(fs, severityWarn, substr) > 0
}

func errors(fs []finding) int {
	return countFindings(fs, severityError, "")
}

func warns(fs []finding) int {
	return countFindings(fs, severityWarn, "")
}

// Test 1: missing required section
func TestValidate_MissingRequiredSection(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "missing-section", "spec.md")
	fs := validate(m)
	// Should have errors for missing sections
	assert.Greater(t, errors(fs), 0, "expected errors for missing sections")
	assert.True(t, hasError(fs, "missing required section"), "expected missing section error")
}

// Test 2a: status off-format with inline parenthetical
func TestValidate_StatusInlineParenthetical(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "status-inline", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "inline qualifier"), "expected inline qualifier error for status with parenthetical")
}

// Test 2b: status with unknown state
func TestValidate_StatusUnknownState(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "status-unknown", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "unknown status"), "expected unknown status error")
}

// Test 3: uncovered REQ
func TestValidate_UncoveredReq(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "uncovered-req", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "REQ-2"), "expected error for uncovered REQ-2")
	assert.True(t, hasError(fs, "no test coverage"), "expected no-coverage message")
}

// Test 4: task without files
func TestValidate_TaskWithoutFiles(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "task-no-files", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "no files:"), "expected error for task with no files")
}

// Test 5a: prod .go task without tests → WARN
func TestValidate_ProdGoTaskNoTests_Warn(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "task-no-tests", "spec.md")
	fs := validate(m)
	assert.True(t, hasWarn(fs, "production .go file"), "expected warn for prod .go without tests")
}

// Test 5b: mixed excluded+non-excluded → WARN
func TestValidate_ProdGoTaskNoTests_Mixed(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "task-no-tests-mixed", "spec.md")
	fs := validate(m)
	assert.True(t, hasWarn(fs, "production .go file"), "expected warn for mixed task with non-excluded prod go file")
}

// Test 5c: task with only excluded files → no warn
func TestValidate_ProdGoTaskOnlyExcluded_NoWarn(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "task-excluded", "spec.md")
	fs := validate(m)
	assert.False(t, hasWarn(fs, "production .go file"), "expected no warn for task with only excluded files")
}

// Test 6a: depends cycle (T1→T2→T1)
func TestValidate_DependsCycle(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "depends-cycle", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "cycle"), "expected cycle detection error")
}

// Test 6b: self-loop (T1→T1)
func TestValidate_DependsSelfLoop(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "depends-selfloop", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "cycle"), "expected self-loop cycle error")
}

// Test 7: depends references nonexistent task
func TestValidate_DependsNonExistent(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "depends-nonexist", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "does not exist"), "expected error for depends on nonexistent task")
}

// Test 8: two tasks in batch share non-shared-additive file
func TestValidate_BatchFileOverlap(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "batch-file-overlap", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "shared.go"), "expected batch overlap error naming the file")
	assert.True(t, hasError(fs, "batch"), "expected batch number in error message")
}

// Test 9: task absent from all batches AND task in two batches → exactly 2 errors
func TestValidate_BatchMembership(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "batch-membership", "spec.md")
	fs := validate(m)
	absentErrs := countFindings(fs, severityError, "not in any batch")
	dupErrs := countFindings(fs, severityError, "appears in")
	assert.Equal(t, 1, absentErrs, "expected exactly 1 absent-from-batch error")
	assert.Equal(t, 1, dupErrs, "expected exactly 1 duplicate-batch error")
}

// Test 10: batch order violates depends
func TestValidate_BatchOrder(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "batch-order", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "earlier batch"), "expected batch order error")
}

// Test 11a: TC-ID missing prefix (slug present)
func TestValidate_TCIDMissingPrefix(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "tc-id-bad", "spec-missing-prefix.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "TC-UC-01"), "expected error for TC without slug prefix")
}

// Test 11b: TC-ID with wrong prefix
func TestValidate_TCIDWrongPrefix(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "tc-id-bad", "spec-wrong-prefix.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "WRONG-TC-UC-01"), "expected error for TC with wrong prefix")
}

// Test 11c: TC-ID with bad type segment
func TestValidate_TCIDBadType(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "tc-id-bad", "spec-bad-type.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "unregistered type"), "expected error for TC with bad type segment")
}

// Test 12a: REQ-ID missing prefix
func TestValidate_REQIDMissingPrefix(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "req-id-bad", "spec-missing-prefix.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "REQ-1"), "expected error for REQ without slug prefix")
}

// Test 12b: REQ-ID with wrong prefix
func TestValidate_REQIDWrongPrefix(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "req-id-bad", "spec-wrong-prefix.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "WRONG-REQ-1"), "expected error for REQ with wrong prefix")
}

// Test 13: TC refs nonexistent REQ
func TestValidate_TCRefInvalid(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "tc-ref-invalid", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "REQ-99"), "expected error for TC referencing nonexistent REQ")
}

// Test 14a: slug-declared spec with NO docs/capabilities ref → WARN
func TestValidate_CapabilityLinked_Missing(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "no-capability", "spec.md")
	fs := validate(m)
	assert.True(t, hasWarn(fs, "docs/capabilities"), "expected warn for missing capability link")
}

// Test 14b: golden spec with capability ref → no capability warn
func TestValidate_CapabilityLinked_Present(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "valid", "golden.md")
	fs := validate(m)
	assert.False(t, hasWarn(fs, "docs/capabilities"), "expected no capability warn when link present")
}

// Test 15: duplicate TC-ID and REQ-ID → error each
func TestValidate_DuplicateIDs(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "dup-ids", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "duplicate req id"), "expected error for duplicate REQ-1")
	assert.True(t, hasError(fs, "duplicate tc id"), "expected error for duplicate TC-UC-01")
}

// Test 16 exit codes are covered in main_test.go

// Test 17 manifest tests are in manifest_test.go

// Test 18: GOLDEN valid synthetic fixture → 0 errors, 0 warns
func TestValidate_GoldenSpec(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "valid", "golden.md")
	fs := validate(m)
	assert.Equal(t, 0, errors(fs), "golden spec should have 0 errors, got: %v", fs)
	assert.Equal(t, 0, warns(fs), "golden spec should have 0 warnings, got: %v", fs)
}

// Test 19a: invalid slug value
func TestValidate_SlugInvalid(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "slug-invalid", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "invalid"), "expected slug invalid error")
}

// Test 19b: duplicate slug
func TestValidate_SlugDuplicate(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "slug-dup", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "duplicate"), "expected duplicate slug error")
}

// Test 19c: valid slug → no slug error
func TestValidate_SlugValid(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "slug-valid", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "slug"), "expected no slug error for valid SDDX slug")
}

// Test 20: Status lines inside fenced blocks → ignored
func TestValidate_FencedStatusIgnored(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "fenced", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "duplicate ## Status"), "fenced status lines should be ignored")
	assert.Equal(t, 1, m.statusLineCount, "only the real status line should be counted")
}

// Test 21a: legacy spec without slug — bare IDs → no slug/prefix/capability findings
func TestValidate_Grandfathered_NoSlug(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "grandfathered", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "slug"), "legacy spec should have no slug errors")
	assert.False(t, hasError(fs, "prefix"), "legacy spec should have no prefix errors")
	assert.False(t, hasWarn(fs, "docs/capabilities"), "legacy spec should have no capability warn")
}

// Test 21b: slug present + bare IDs → error per bare ID
func TestValidate_Grandfathered_WithSlug(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "grandfathered-with-slug", "spec.md")
	fs := validate(m)
	// REQ-1 and TC-UC-01 both lack the SDDX prefix → an error naming each (not just a count)
	assert.True(t, hasError(fs, "REQ-1"), "bare REQ-1 must be flagged (missing slug prefix)")
	assert.True(t, hasError(fs, "TC-UC-01"), "bare TC-UC-01 must be flagged (missing slug prefix)")
}

// Test 22: registered types accepted; unregistered → error
func TestValidate_TCTypeUnregistered(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "tc-type-unregistered", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "unregistered type"), "expected error for unregistered TC type ZZ")
}

// Test 23: duplicate ## Status: lines
func TestValidate_DuplicateStatus(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "status-dup", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "duplicate ## Status"), "expected duplicate status error")
}

// Test 24: shared-additive PASS — two tasks share file in shared-additive → no batchFileOverlap error
func TestValidate_BatchSharedAdditive_NoOverlap(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "batch-sharedadditive", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "shared.go"), "shared-additive file should not trigger overlap error")
	assert.Equal(t, 0, errors(fs), "a shared-additive fixture must have zero errors")
}

// Test 25a: valid (no-test: reason) → no error
func TestValidate_ReqNoTestValid(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "req-no-test-empty", "spec.md")
	fs := validate(m)
	// REQ-1 has empty no-test → error; REQ-2 has valid reason → no error for REQ-2
	assert.True(t, hasError(fs, "REQ-1"), "expected error for empty no-test annotation")
	assert.False(t, hasError(fs, "REQ-2"), "expected no error for valid no-test annotation")
}

// Test 25b: empty (no-test:) → ERROR
func TestValidate_ReqNoTestEmpty(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "req-no-test-empty", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "empty reason"), "expected error for empty no-test reason")
}

// Test 26 malformed capability → manifest_test.go

// Test 27 main exit codes → main_test.go

// Test 28: empty ## Execution Log (heading only) → no requiredSections ERROR
func TestValidate_EmptyExecutionLog(t *testing.T) {
	t.Parallel()
	// The golden spec has an empty Execution Log
	m := loadSpec(t, "valid", "golden.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "Execution Log"), "empty Execution Log heading should satisfy required section")
}

// Test 29a: SUPERSEDED status → 0 status errors
func TestValidate_StatusSuperseded(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "superseded", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "status"), "SUPERSEDED should be a valid status")
}

// Test 29b: ARCHIVED status → 0 status errors
func TestValidate_StatusArchived(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "archived", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "status"), "ARCHIVED should be a valid status")
}

// Test 30: task files=[internal/domain/user/server.go] no tests → 1 WARN (not excluded)
func TestValidate_DomainServerGo_Warn(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "task-no-tests", "spec-domain-server.md")
	fs := validate(m)
	assert.True(t, hasWarn(fs, "production .go file"), "internal/domain/user/server.go should warn (not excluded)")
}

// Test 31: task files=[cmd/api/server.go, cmd/api/container.go, cmd/api/router.go] → 0 WARN
func TestValidate_AllExcludedAPIFiles_NoWarn(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "task-excluded", "spec-all-excluded.md")
	fs := validate(m)
	assert.Equal(t, 0, warns(fs), "all-excluded API files should produce 0 warnings")
}

// Test 32: task files include a .go under /migration/ → 0 WARN
func TestValidate_MigrationGo_NoWarn(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "taskfiles-migration", "spec.md")
	fs := validate(m)
	assert.False(t, hasWarn(fs, "production .go file"), "migration .go files should not trigger test warn")
}

// Test 33: REQ with valid (no-test:) AND a TC → 0 ERROR
func TestValidate_ReqNoTestWithTC_NoError(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "req-notestvalid-has-tc", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "REQ-1"), "REQ with valid no-test annotation should be exempt even if TC exists")
}

// Test 34: TC-ID with single-digit suffix → ERROR
func TestValidate_TCSingleDigitSuffix(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "tc-id-bad", "spec-single-digit.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "at least 2 digits"), "single-digit TC suffix should error")
}

// Test 35: legacy (no slug) with no capability ref → 0 WARN
func TestValidate_LegacyNoCapability_NoWarn(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "legacy-no-capability", "spec.md")
	fs := validate(m)
	assert.False(t, hasWarn(fs, "docs/capabilities"), "legacy spec without slug should not warn about capability link")
}

// Test 37: spec with NO ## Status: line → ≥1 statusHeader ERROR
func TestValidate_NoStatusLine_Error(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "status-none", "spec.md")
	fs := validate(m)
	// statusHeader fires independently of requiredSections — both must report the missing status.
	assert.True(t, hasError(fs, "no ## Status: line found"), "statusHeader must report the absence")
	assert.True(t, hasError(fs, "missing required section: Status"), "requiredSections must also report it")
}

// Test 38: ~~~go tilde fence containing ## Status: WIP → ignored
func TestValidate_TildeFenceStatusIgnored(t *testing.T) {
	t.Parallel()
	content := "## Status: DRAFT\n\n## Context\n\nx\n\n~~~go\n## Status: WIP\n~~~\n\n## Requirements\n\n- [ ] REQ-1: r\n\n## Test Plan\n\n| TC-ID | REQ | Category | Description | Expected |\n| --- | --- | --- | --- | --- |\n| TC-UC-01 | REQ-1 | happy | x | ok |\n\n## Design\n\nd\n\n## Tasks\n\n- [ ] TASK-1: t\n  - files: internal/domain/x/entity.go\n  - tests: TC-UC-01\n\n## Parallel Batches\n\nBatch 1: [TASK-1]\n\n## Validation Criteria\n\nAll pass.\n\n## Execution Log\n\n"
	m, parseErr := parseSpec(content)
	require.NoError(t, parseErr)
	assert.Equal(t, 1, m.statusLineCount, "tilde-fenced status line should be ignored")
	fs := validate(m)
	assert.False(t, hasError(fs, "duplicate"), "no duplicate status error expected when second status is inside tilde fence")
}

// Additional edge cases for completeness

// validateIDFormat for all registered types
func TestValidate_RegisteredTCTypes(t *testing.T) {
	t.Parallel()
	types := []string{"D", "UC", "E2E", "S", "SH", "CT"}
	for _, tcType := range types {
		t.Run("type_"+tcType, func(t *testing.T) {
			t.Parallel()
			content := buildSimpleSpecWithTC("SDDX-TC-" + tcType + "-01")
			m, parseErr := parseSpec(content)
			require.NoError(t, parseErr)
			fs := validateIDFormat(m)
			assert.False(t, hasError(fs, "unregistered"), "type %s should be registered", tcType)
		})
	}
}

// buildSimpleSpecWithTC creates a minimal spec content with the given TC ID.
func buildSimpleSpecWithTC(tcID string) string {
	return "## Slug: SDDX\n\n## Status: DRAFT\n\n## Context\n\nx\n\n## Requirements\n\n- [ ] SDDX-REQ-1: r\n\n## Test Plan\n\n| TC-ID | REQ | Category | Description | Expected |\n| --- | --- | --- | --- | --- |\n| " + tcID + " | SDDX-REQ-1 | happy | x | ok |\n\n## Design\n\nd\n\n## Tasks\n\n- [ ] TASK-1: t\n  - files: internal/domain/x/entity.go\n  - tests: " + tcID + "\n\n## Parallel Batches\n\nBatch 1: [TASK-1]\n\n## Validation Criteria\n\nAll pass.\n\n## Execution Log\n\n"
}

// TC-01: dangling tests: ref → ERROR naming TC + task
func TestValidate_TestsRefValid_Dangling(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "tests-dangling", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "TDGL-TC-UC-99"), "error must name the undeclared TC")
	assert.True(t, hasError(fs, "TASK-1"), "error must name the task")
	assert.True(t, hasError(fs, "undeclared TC"), "error must say 'undeclared TC'")
}

// TC-02: orphan TC → ERROR naming the TC
func TestValidate_TCReferenced_Orphan(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "tc-orphan", "spec.md")
	fs := validate(m)
	assert.True(t, hasError(fs, "TORP-TC-UC-02"), "error must name the orphan TC")
	assert.True(t, hasError(fs, "referenced by no task"), "error must say 'referenced by no task'")
}

// TC-03: golden → 0 round-trip findings
func TestValidate_GoldenRoundTrip(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "valid", "golden.md")
	fs := validate(m)
	assert.Equal(t, 0, errors(fs), "golden spec must have 0 errors")
	assert.Equal(t, 0, warns(fs), "golden spec must have 0 warns")
}

// TC-04: grandfathered no-slug + orphan → skipped (no error from round-trip validators)
func TestValidate_Grandfathered_RoundTripSkipped(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "grandfathered-orphan", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "undeclared TC"), "no-slug spec must skip testsRefValid")
	assert.False(t, hasError(fs, "referenced by no task"), "no-slug spec must skip tcReferenced")
}

// TC-05: smoke TC referenced only by TASK-SMOKE → not orphan
func TestValidate_SmokeTCReferenced_ViaTaskSmoke(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "smoke-referenced", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "SMKR-TC-S-01"), "SMKR-TC-S-01 referenced by TASK-SMOKE → not orphan")
	assert.False(t, hasError(fs, "referenced by no task"), "no orphan error expected")
}

// TC-16: roundtrip-only fixture fails ONLY tcReferenced → exit 1; golden → exit 0
func TestRun_RoundtripOnly_ExitCode1(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", filepath.Join("testdata", "roundtrip-only", "spec.md")})
	assert.Equal(t, 1, code, "roundtrip-only (one orphan TC) must exit 1")
	// Golden must still exit 0
	golden := filepath.Join("testdata", "valid", "golden.md")
	assert.Equal(t, 0, run([]string{"validate-spec", golden}), "golden must exit 0")
}

// TC-17: spec with TASK-SMOKE → two distinct tasks, no metadata bleed
func TestParseSpec_NamedTasks_Distinct(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "smoke-referenced", "spec.md")
	require.Len(t, m.tasks, 2, "should have TASK-1 and TASK-SMOKE")
	// TASK-1 must not have TASK-SMOKE's tests
	assert.Equal(t, "TASK-1", m.tasks[0].id)
	assert.Equal(t, "TASK-SMOKE", m.tasks[1].id)
	assert.NotContains(t, m.tasks[0].tests, "SMKR-TC-S-01", "TASK-1 must not bleed TASK-SMOKE's tests")
}

// TC-19: numbered task mentioning TASK-SMOKE in prose → no spurious task
func TestParseSpec_ProseTaskName_NoSpuriousTask(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "prose-task", "spec.md")
	require.Len(t, m.tasks, 1, "only TASK-1 should be parsed; TASK-SMOKE in prose must NOT create a task")
	assert.Equal(t, "TASK-1", m.tasks[0].id)
}

// TC-20: TC referenced by two tasks → not orphan, 0 round-trip findings
func TestValidate_TCMultiRef_NotOrphan(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "tc-multiref", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "TMRF-TC-UC-01"), "TC referenced by two tasks must not be orphan")
	assert.Equal(t, 0, errors(fs), "tc-multiref must have 0 errors")
}

// TC-22: task with `- files: (none — execution only)` → no batchFileOverlap, no taskHasFiles error
func TestValidate_ExecOnly_NoErrors(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "task-execonly", "spec.md")
	fs := validate(m)
	assert.False(t, hasError(fs, "TASK-SMOKE"), "execOnly task must not trigger taskHasFiles error")
	assert.False(t, hasError(fs, "no files:"), "no 'no files:' error for execOnly task")
	assert.Equal(t, 0, errors(fs), "task-execonly must have 0 errors")
}

func TestValidate_SeverityString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "warning", severityWarn.String())
	assert.Equal(t, "error", severityError.String())
}

func TestValidate_ExcludedByPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     string
		excluded bool
	}{
		{"cmd/api/server.go", true},
		{"cmd/api/container.go", true},
		{"cmd/api/router.go", true},
		{"internal/domain/user/server.go", false},
		{"internal/infrastructure/db/postgres/migration/20240101.go", true},
		{"internal/domain/x/entity.go", false},
		{"internal/domain/x/entity_test.go", true},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.excluded, excludedByPath(tc.path), "path: %s", tc.path)
		})
	}
}
