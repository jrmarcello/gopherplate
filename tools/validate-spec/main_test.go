package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test 16/27: exit code tests via run()

// ERROR spec → exit 1
func TestRun_ErrorSpec_ExitCode1(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "uncovered-req", "spec.md")
	code := run([]string{"validate-spec", path})
	assert.Equal(t, 1, code, "ERROR spec should exit 1")
}

// A clean grandfathered spec (no ## Slug:) with no errors exits 0.
// The warn-only → exit 0 branch is covered by TestRun_WarnOnlySpec_ExitsZero.
func TestRun_GrandfatheredSpec_ExitCode0(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", filepath.Join("testdata", "grandfathered", "spec.md")})
	assert.Equal(t, 0, code, "a clean grandfathered spec should exit 0")
}

// golden spec → exit 0
func TestRun_GoldenSpec_ExitCode0(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "valid", "golden.md")
	code := run([]string{"validate-spec", path})
	assert.Equal(t, 0, code, "golden spec should exit 0")
}

// Test 27: nonexistent file → exit 2
func TestRun_NonexistentFile_ExitCode2(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", "testdata/nonexistent/spec.md"})
	assert.Equal(t, 2, code, "nonexistent file should exit 2")
}

// no args → exit 2
func TestRun_NoArgs_ExitCode2(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec"})
	assert.Equal(t, 2, code, "no args should exit 2")
}

// manifest with no dir → defaults to docs/capabilities, which is absent in the test
// working dir, so the read fails → exit 2.
func TestRun_ManifestNoDir_ExitCode2(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", "manifest"})
	assert.Equal(t, 2, code, "manifest with a missing caps dir should exit 2")
}

// manifest subcommand with valid dir → exit 0
func TestRun_ManifestValidDir_ExitCode0(t *testing.T) {
	t.Parallel()
	capsDir := filepath.Join("testdata", "manifest", "caps")
	code := run([]string{"validate-spec", "manifest", capsDir})
	assert.Equal(t, 0, code, "manifest with valid dir should exit 0")
}

// manifest with nonexistent dir → exit 2
func TestRun_ManifestNonexistentDir_ExitCode2(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", "manifest", "testdata/nonexistent-caps"})
	assert.Equal(t, 2, code, "manifest with nonexistent dir should exit 2")
}

// files subcommand: nonexistent spec → exit 2
func TestRun_Files_NonexistentSpec_ExitCode2(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", "files", "testdata/nonexistent/spec.md"})
	assert.Equal(t, 2, code)
}

// files subcommand: valid spec → exit 0
func TestRun_Files_ValidSpec_ExitCode0(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", "files", filepath.Join("testdata", "files-union", "spec.md")})
	assert.Equal(t, 0, code)
}

// capabilities subcommand: nonexistent dir → exit 2
func TestRun_Capabilities_NonexistentDir_ExitCode2(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", "capabilities", "testdata/nonexistent-caps"})
	assert.Equal(t, 2, code)
}

// bootstrap-capability subcommand: valid pkg → exit 0
func TestRun_BootstrapCapability_ValidPkg_ExitCode0(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", "bootstrap-capability", filepath.Join("testdata", "bootstrap", "witherrors")})
	assert.Equal(t, 0, code)
}

// bootstrap-capability subcommand: missing arg → exit 2
func TestRun_BootstrapCapability_MissingArg_ExitCode2(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", "bootstrap-capability"})
	assert.Equal(t, 2, code)
}

// help flag → exit 0
func TestRun_Help_ExitCode0(t *testing.T) {
	t.Parallel()
	code := run([]string{"validate-spec", "--help"})
	assert.Equal(t, 0, code, "--help should exit 0")
}

// multiple spec files: one clean, one with errors → exit 1
func TestRun_MultipleSpecs_OneError(t *testing.T) {
	t.Parallel()
	golden := filepath.Join("testdata", "valid", "golden.md")
	errorSpec := filepath.Join("testdata", "uncovered-req", "spec.md")
	code := run([]string{"validate-spec", golden, errorSpec})
	assert.Equal(t, 1, code, "if any spec has errors, overall exit should be 1")
}
