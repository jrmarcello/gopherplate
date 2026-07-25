package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 17a: renderManifest lists each capability
func TestRenderManifest_ListsCapabilities(t *testing.T) {
	t.Parallel()
	caps := []capability{
		{name: "alpha", slug: "ALPHA", status: "STABLE", guaranteedReqs: []string{"ALPHA-REQ-1", "ALPHA-REQ-2"}},
		{name: "beta", slug: "BETA", status: "EXPERIMENTAL"},
	}
	out := renderManifest(caps)
	assert.Contains(t, out, "ALPHA")
	assert.Contains(t, out, "BETA")
	assert.Contains(t, out, "STABLE")
	assert.Contains(t, out, "EXPERIMENTAL")
	assert.Contains(t, out, "ALPHA-REQ-1")
	assert.Contains(t, out, "ALPHA-REQ-2")
}

// Test 17b: empty directory → valid empty manifest (just header)
func TestRenderManifest_EmptyDir(t *testing.T) {
	t.Parallel()
	out := renderManifest(nil)
	assert.Contains(t, out, "Capabilities Manifest")
	assert.Contains(t, out, "| Name |")
}

// Test 17c: called twice → identical string (deterministic)
func TestRenderManifest_Deterministic(t *testing.T) {
	t.Parallel()
	caps := []capability{
		{name: "zzz", slug: "ZZZ", status: "STABLE"},
		{name: "aaa", slug: "AAA", status: "EXPERIMENTAL"},
	}
	out1 := renderManifest(caps)
	out2 := renderManifest(caps)
	assert.Equal(t, out1, out2, "renderManifest must be deterministic")
	// Also verify sorted order: AAA before ZZZ
	idxAAA := strings.Index(out1, "AAA")
	idxZZZ := strings.Index(out1, "ZZZ")
	assert.Less(t, idxAAA, idxZZZ, "capabilities should be sorted by slug")
}

// Test 26: malformed cap doc → <missing> placeholder + stderr warning, no crash
func TestParseCapability_Malformed(t *testing.T) {
	t.Parallel()
	content := "# Malformed\n\nNo Slug or Status here.\n"
	cap, ok := parseCapability("malformed", content)
	assert.False(t, ok, "malformed capability should return ok=false")
	assert.Equal(t, "malformed", cap.name)
}

func TestRenderManifest_WithMalformed(t *testing.T) {
	t.Parallel()
	caps := []capability{
		{name: "alpha", slug: "ALPHA", status: "STABLE"},
		{name: "malformed"}, // slug and status empty → <missing>
	}
	out := renderManifest(caps)
	assert.Contains(t, out, "<missing>", "malformed capability should show <missing>")
	assert.Contains(t, out, "ALPHA", "valid capability should still appear")
}

// Test 36: manifest --write writes to file, content==renderManifest, re-run no diff
func TestRunManifest_Write(t *testing.T) {
	t.Parallel()
	capsDir := filepath.Join("testdata", "manifest", "caps")
	tmpDir := t.TempDir()
	writePath := filepath.Join(tmpDir, "MANIFEST.md")

	manifestErr := runManifest(capsDir, writePath)
	require.NoError(t, manifestErr)

	// File should exist and be non-empty
	data, readErr := os.ReadFile(writePath)
	require.NoError(t, readErr)
	content := string(data)
	assert.Contains(t, content, "Capabilities Manifest")
	assert.Contains(t, content, "ALPHA")
	assert.Contains(t, content, "BETA")

	// Re-run should produce identical content
	writeErr2 := runManifest(capsDir, writePath)
	require.NoError(t, writeErr2)
	data2, readErr2 := os.ReadFile(writePath)
	require.NoError(t, readErr2)
	assert.Equal(t, content, string(data2), "second run should be identical to first")
}

// Test 36b: runManifest stdout (no write) reads caps dir correctly
func TestRunManifest_CapsDir(t *testing.T) {
	t.Parallel()
	capsDir := filepath.Join("testdata", "manifest", "caps")
	// Just verify it doesn't error
	manifestErr := runManifest(capsDir, "")
	require.NoError(t, manifestErr)
}

// Malformed cap in real dir produces warning to stderr but no crash
func TestRunManifest_MalformedCapInDir(t *testing.T) {
	t.Parallel()
	capsDir := filepath.Join("testdata", "manifest", "caps")
	tmpDir := t.TempDir()
	writePath := filepath.Join(tmpDir, "out.md")
	// malformed.md is in the caps dir — should produce a warning but still succeed
	manifestErr := runManifest(capsDir, writePath)
	require.NoError(t, manifestErr, "malformed capability should not cause error")
	data, readErr := os.ReadFile(writePath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "<missing>", "malformed capability should have <missing> in output")
}

// runManifest skips README.md and TEMPLATE.md (non-capability docs), not just MANIFEST.md.
func TestRunManifest_SkipsNonCapabilityFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "user.md"),
		[]byte("# Capability: user\n\n## Slug: USER\n\n## Status: Active\n\n## Guaranteed Requirements\n\n- USER-REQ-1: x\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"),
		[]byte("# Capability Docs\n\nindex, not a capability\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "TEMPLATE.md"),
		[]byte("# Capability: name\n\n## Slug: PLACEHOLDER\n\n## Status: Active\n"), 0o644))

	out := filepath.Join(dir, "MANIFEST.md")
	require.NoError(t, runManifest(dir, out))
	data, readErr := os.ReadFile(out)
	require.NoError(t, readErr)
	content := string(data)
	assert.Contains(t, content, "USER", "the real capability must be listed")
	assert.NotContains(t, content, "README", "README.md must be skipped")
	assert.NotContains(t, content, "PLACEHOLDER", "TEMPLATE.md must be skipped")
}
