package boilerplate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyProject(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Source files to create — covers all inclusion/exclusion scenarios.
	files := map[string]string{
		// Should be COPIED (always included)
		"cmd/api/main.go":                "package main\n",
		"internal/domain/user/entity.go": "package user\n",
		"pkg/cache/cache.go":             "package cache\n",
		"Makefile":                       "APP_NAME := gopherplate\n",
		".gitignore":                     ".env\nbin/\n",
		"deploy/base/deployment.yaml":    "name: gopherplate\n",
		"tests/e2e/user_test.go":         "package e2e\n",

		// Should be COPIED (newly included — REQ-1, REQ-2, REQ-3, REQ-4, REQ-5)
		".claude/settings.json":            `{"permissions":{}}`,
		".claude/hooks/guard-bash.sh":      "#!/bin/bash\n",
		".claude/rules/go-conventions.md":  "# Go conventions\n",
		".claude/agents/code-reviewer.md":  "# Code reviewer\n",
		".claude/skills/validate/SKILL.md": "# Validate\n",
		".devcontainer/devcontainer.json":  `{"name":"gopherplate"}`,
		".devcontainer/init-firewall.sh":   "#!/bin/bash\n# gopherplate\n",
		".devcontainer/Dockerfile":         "FROM node\n",
		".specs/TEMPLATE.md":               "# Template\n",
		".specs/.gitkeep":                  "",
		".specs/.gitignore":                "*.active.md\n",
		".specs/dx-sdd-tdd-parallelism.md": "# DX spec\n",
		".specs/my-feature.active.md":      "active\n",
		"CLAUDE.md":                        "# Claude\n",
		"AGENTS.md":                        "# Agents\n",
		"CONTRIBUTING.md":                  "# Contributing\n",
		".markdownlint.json":               "{}",

		// Should be COPIED (changelog tooling — reset in post-processing)
		"cliff.toml":   "[changelog]",
		"CHANGELOG.md": "# Changelog",

		// Should NOT be copied (excluded)
		".git/config":                   "[core]\n",
		"cmd/cli/main.go":               "package main\n",
		".claude/worktrees/abc/file.go": "package abc\n",
		".claude/settings.local.json":   `{"local":true}`,
		"bin/api":                       "binary",
		".github/workflows/ci.yml":      "name: CI",
		".env":                          "SECRET=val",
		"roadmap.md":                    "# Roadmap",
		"docs/guides/template-cli.md":   "# CLI guide",
		"docs/modules/something.md":     "# mod",
		"tests/coverage/cover.html":     "<html>",
		"tests/load/results/run1.json":  "{}",
	}

	for relPath, content := range files {
		absPath := filepath.Join(srcDir, relPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o750))
		require.NoError(t, os.WriteFile(absPath, []byte(content), 0o600))
	}

	copyErr := CopyProject(srcDir, dstDir)
	require.NoError(t, copyErr)

	// TC-U-01 to TC-U-17: Files that SHOULD exist in destination
	shouldExist := []string{
		"cmd/api/main.go",
		"internal/domain/user/entity.go",
		"pkg/cache/cache.go",
		"Makefile",
		".gitignore",
		"deploy/base/deployment.yaml",
		"tests/e2e/user_test.go",
		// REQ-1: .claude/ included (except worktrees/settings.local)
		".claude/settings.json",            // TC-U-01
		".claude/hooks/guard-bash.sh",      // TC-U-02
		".claude/rules/go-conventions.md",  // TC-U-03
		".claude/agents/code-reviewer.md",  // TC-U-04
		".claude/skills/validate/SKILL.md", // TC-U-05
		// REQ-2: .devcontainer/ included
		".devcontainer/devcontainer.json", // TC-U-08
		".devcontainer/init-firewall.sh",  // TC-U-09
		".devcontainer/Dockerfile",
		// REQ-3: .specs/ included (template structure)
		".specs/TEMPLATE.md", // TC-U-10
		".specs/.gitkeep",    // TC-U-11
		".specs/.gitignore",
		// REQ-3 note: spec files ARE copied by CopyProject (cleanup happens in new.go step 9)
		".specs/dx-sdd-tdd-parallelism.md",
		".specs/my-feature.active.md",
		// REQ-4: docs included
		"CLAUDE.md",       // TC-U-14
		"AGENTS.md",       // TC-U-15
		"CONTRIBUTING.md", // TC-U-16
		// REQ-5: markdownlint included
		".markdownlint.json", // TC-U-17
		// Changelog tooling (copied, reset in post-processing)
		"cliff.toml",
		"CHANGELOG.md",
	}
	for _, rel := range shouldExist {
		_, statErr := os.Stat(filepath.Join(dstDir, rel))
		assert.NoError(t, statErr, "expected %s to exist in destination", rel)
	}

	// TC-U-06, TC-U-07, TC-U-18, TC-U-19: Files that should NOT exist
	shouldNotExist := []string{
		".git/config",
		"cmd/cli/main.go",
		".claude/worktrees/abc/file.go", // TC-U-06
		".claude/settings.local.json",   // TC-U-07
		"bin/api",
		".github/workflows/ci.yml",
		".env",
		"roadmap.md",                  // TC-U-18
		"docs/guides/template-cli.md", // TC-U-19
		"docs/modules/something.md",
		"tests/coverage/cover.html",
		"tests/load/results/run1.json",
	}
	for _, rel := range shouldNotExist {
		_, statErr := os.Stat(filepath.Join(dstDir, rel))
		assert.True(t, os.IsNotExist(statErr), "expected %s to be excluded from destination", rel)
	}

	// Verify content is preserved
	content, readErr := os.ReadFile(filepath.Join(dstDir, "Makefile"))
	require.NoError(t, readErr)
	assert.Equal(t, "APP_NAME := gopherplate\n", string(content))
}

// TC-U-20, TC-U-23: shouldExclude returns correct results for all rules
func TestShouldExclude(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		expected bool
	}{
		// Excluded
		{name: "git directory", relPath: ".git/config", expected: true},
		{name: "git root", relPath: ".git", expected: true},
		{name: "cli cmd", relPath: "cmd/cli/main.go", expected: true},
		{name: "claude worktrees", relPath: ".claude/worktrees/abc", expected: true},
		{name: "claude settings local", relPath: ".claude/settings.local.json", expected: true},
		{name: "bin artifacts", relPath: "bin/api", expected: true},
		{name: "github ci", relPath: ".github/workflows/ci.yml", expected: true},
		{name: "cliff toml", relPath: "cliff.toml", expected: false},
		{name: "changelog", relPath: "CHANGELOG.md", expected: false},
		{name: "roadmap", relPath: "roadmap.md", expected: true},
		{name: "template cli guide", relPath: "docs/guides/template-cli.md", expected: true},
		{name: "docs modules", relPath: "docs/modules/something.md", expected: true},
		{name: "env file", relPath: ".env", expected: true},
		{name: "test coverage", relPath: "tests/coverage/cover.html", expected: true},
		{name: "load results", relPath: "tests/load/results/run1.json", expected: true},

		// Included (NOT excluded)
		{name: "regular go file", relPath: "cmd/api/main.go", expected: false},
		{name: "domain file", relPath: "internal/domain/user/entity.go", expected: false},
		{name: "gitignore", relPath: ".gitignore", expected: false},
		{name: "env example", relPath: ".env.example", expected: false},
		{name: "claude settings json", relPath: ".claude/settings.json", expected: false},
		{name: "claude hooks", relPath: ".claude/hooks/guard-bash.sh", expected: false},
		{name: "claude rules", relPath: ".claude/rules/go-conventions.md", expected: false},
		{name: "claude agents", relPath: ".claude/agents/code-reviewer.md", expected: false},
		{name: "claude skills", relPath: ".claude/skills/validate/SKILL.md", expected: false},
		{name: "devcontainer json", relPath: ".devcontainer/devcontainer.json", expected: false},
		{name: "devcontainer firewall", relPath: ".devcontainer/init-firewall.sh", expected: false},
		{name: "specs template", relPath: ".specs/TEMPLATE.md", expected: false},
		{name: "specs gitkeep", relPath: ".specs/.gitkeep", expected: false},
		{name: "specs feature", relPath: ".specs/my-feature.md", expected: false},
		{name: "CLAUDE.md", relPath: "CLAUDE.md", expected: false},
		{name: "AGENTS.md", relPath: "AGENTS.md", expected: false},
		{name: "CONTRIBUTING.md", relPath: "CONTRIBUTING.md", expected: false},
		{name: "markdownlint", relPath: ".markdownlint.json", expected: false},
		{name: "other docs guide", relPath: "docs/guides/architecture.md", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldExclude(tt.relPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}
