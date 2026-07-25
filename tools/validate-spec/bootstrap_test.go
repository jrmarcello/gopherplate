package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TC-13: bootstrap with sentinels + entity_test.go → skeleton lists non-test .go ONLY + Error-Contract
func TestRenderBootstrap_WithErrors(t *testing.T) {
	t.Parallel()
	files := []string{"entity.go"}
	errs := []string{"ErrFixtureConflict", "ErrFixtureNotFound"}
	output := renderBootstrap("witherrors", files, errs, "2026-01-01", "abc1234")
	assert.Contains(t, output, "# Capability: witherrors")
	assert.Contains(t, output, "## Slug: TODO")
	assert.Contains(t, output, "## Code")
	assert.Contains(t, output, "- entity.go")
	assert.NotContains(t, output, "entity_test.go", "test files must be excluded")
	assert.Contains(t, output, "- ErrFixtureConflict")
	assert.Contains(t, output, "- ErrFixtureNotFound")
	assert.Contains(t, output, "## Guarantees (current truth)")
	assert.Contains(t, output, "TODO")
	assert.Contains(t, output, "## Error Contract")
	assert.Contains(t, output, "Last-verified: 2026-01-01 (abc1234)")
}

// TC-14: bootstrap no sentinels → empty Error-Contract, no crash
func TestRenderBootstrap_NoErrors(t *testing.T) {
	t.Parallel()
	output := renderBootstrap("noerrors", []string{"entity.go"}, nil, "2026-01-01", "abc1234")
	assert.Contains(t, output, "## Error Contract")
	assert.NotContains(t, output, "ErrFixture")
}

// TC-15: runBootstrap nonexistent pkg → exit 2
func TestRunBootstrap_NonexistentPkg(t *testing.T) {
	t.Parallel()
	code := runBootstrap(filepath.Join("testdata", "bootstrap", "nonexistent"))
	assert.Equal(t, 2, code, "nonexistent pkg dir → exit 2")
}

// TC-13 integration: runBootstrap over witherrors dir → non-test .go only + sentinels
func TestRunBootstrap_WithErrors_Integration(t *testing.T) {
	t.Parallel()
	code := runBootstrap(filepath.Join("testdata", "bootstrap", "witherrors"))
	assert.Equal(t, 0, code)
}

// TC-14 integration: runBootstrap over noerrors dir → exit 0
func TestRunBootstrap_NoErrors_Integration(t *testing.T) {
	t.Parallel()
	code := runBootstrap(filepath.Join("testdata", "bootstrap", "noerrors"))
	assert.Equal(t, 0, code)
}

// grepSentinels correctly finds Err variables
func TestGrepSentinels(t *testing.T) {
	t.Parallel()
	sentinels, err := grepSentinels(filepath.Join("testdata", "bootstrap", "witherrors", "entity.go"))
	require.NoError(t, err)
	assert.Contains(t, sentinels, "ErrFixtureNotFound")
	assert.Contains(t, sentinels, "ErrFixtureConflict")
	assert.Len(t, sentinels, 2)
}
