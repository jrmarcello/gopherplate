package boilerplate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyProject(t *testing.T) {
	// Create a fake source project tree.
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Source files to create.
	files := map[string]string{
		"cmd/api/main.go":                 "package main\n",
		"internal/domain/user/entity.go":  "package user\n",
		"pkg/cache/cache.go":              "package cache\n",
		"Makefile":                        "APP_NAME := go-boilerplate\n",
		".gitignore":                      ".env\nbin/\n",
		".git/config":                     "[core]\n",
		"cmd/cli/main.go":                 "package main\n",
		"internal/domain/role/entity.go":  "package role\n",
		".claude/rules/go.md":             "# rules\n",
		"CLAUDE.md":                       "# claude\n",
		"AGENTS.md":                       "# agents\n",
		".specs/.gitkeep":                 "",
		".devcontainer/devcontainer.json": "{}",
		"bin/api":                         "binary",
		".github/workflows/ci.yml":        "name: CI",
		"cliff.toml":                      "[changelog]",
		"CHANGELOG.md":                    "# Changelog",
		"CONTRIBUTING.md":                 "# Contributing",
		".env":                            "SECRET=val",
		".markdownlint.json":              "{}",
		"docs/modules/something.md":       "# mod",
		"tests/coverage/cover.html":       "<html>",
		"tests/load/results/run1.json":    "{}",
		"tests/e2e/user_test.go":          "package e2e\n",
		"deploy/base/deployment.yaml":     "name: go-boilerplate\n",
	}

	for relPath, content := range files {
		absPath := filepath.Join(srcDir, relPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o750))
		require.NoError(t, os.WriteFile(absPath, []byte(content), 0o600))
	}

	// Run CopyProject.
	copyErr := CopyProject(srcDir, dstDir)
	require.NoError(t, copyErr)

	// Should be copied.
	shouldExist := []string{
		"cmd/api/main.go",
		"internal/domain/user/entity.go",
		"pkg/cache/cache.go",
		"Makefile",
		".gitignore",
		"deploy/base/deployment.yaml",
		"tests/e2e/user_test.go",
	}
	for _, rel := range shouldExist {
		_, statErr := os.Stat(filepath.Join(dstDir, rel))
		assert.NoError(t, statErr, "expected %s to exist in destination", rel)
	}

	// Should NOT be copied (excluded).
	shouldNotExist := []string{
		".git/config",
		"cmd/cli/main.go",
		".claude/rules/go.md",
		"CLAUDE.md",
		"AGENTS.md",
		".specs/.gitkeep",
		".devcontainer/devcontainer.json",
		"bin/api",
		".github/workflows/ci.yml",
		"cliff.toml",
		"CHANGELOG.md",
		"CONTRIBUTING.md",
		".env",
		".markdownlint.json",
		"docs/modules/something.md",
		"tests/coverage/cover.html",
		"tests/load/results/run1.json",
	}
	for _, rel := range shouldNotExist {
		_, statErr := os.Stat(filepath.Join(dstDir, rel))
		assert.True(t, os.IsNotExist(statErr), "expected %s to be excluded from destination", rel)
	}

	// Verify content is preserved.
	content, readErr := os.ReadFile(filepath.Join(dstDir, "Makefile"))
	require.NoError(t, readErr)
	assert.Equal(t, "APP_NAME := go-boilerplate\n", string(content))
}

func TestShouldExclude(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		expected bool
	}{
		{name: "git directory", relPath: ".git/config", expected: true},
		{name: "git root", relPath: ".git", expected: true},
		{name: "boilerplate cmd", relPath: "cmd/cli/main.go", expected: true},
		{name: "specs dir", relPath: ".specs/template-cli.md", expected: true},
		{name: "claude config", relPath: ".claude/rules/go.md", expected: true},
		{name: "claude md", relPath: "CLAUDE.md", expected: true},
		{name: "agents md", relPath: "AGENTS.md", expected: true},
		{name: "env file", relPath: ".env", expected: true},
		{name: "regular go file", relPath: "cmd/api/main.go", expected: false},
		{name: "domain file", relPath: "internal/domain/user/entity.go", expected: false},
		{name: "gitignore", relPath: ".gitignore", expected: false},
		{name: "env example", relPath: ".env.example", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldExclude(tt.relPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}
