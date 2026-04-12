package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteModulePath(t *testing.T) {
	t.Run("replaces module path in .go files", func(t *testing.T) {
		dir := t.TempDir()
		goFile := filepath.Join(dir, "main.go")
		content := `package main

import "github.com/jrmarcello/gopherplate/internal/domain"

func main() {}
`
		writeErr := os.WriteFile(goFile, []byte(content), 0o644)
		require.NoError(t, writeErr)

		rewriteErr := RewriteModulePath(dir, "github.com/jrmarcello/gopherplate", "github.com/org/my-service")
		require.NoError(t, rewriteErr)

		result, readErr := os.ReadFile(goFile)
		require.NoError(t, readErr)
		assert.Contains(t, string(result), "github.com/org/my-service/internal/domain")
		assert.NotContains(t, string(result), "github.com/jrmarcello/gopherplate")
	})

	t.Run("replaces module path in go.mod", func(t *testing.T) {
		dir := t.TempDir()
		goMod := filepath.Join(dir, "go.mod")
		content := `module github.com/jrmarcello/gopherplate

go 1.25.0
`
		writeErr := os.WriteFile(goMod, []byte(content), 0o644)
		require.NoError(t, writeErr)

		rewriteErr := RewriteModulePath(dir, "github.com/jrmarcello/gopherplate", "github.com/org/my-service")
		require.NoError(t, rewriteErr)

		result, readErr := os.ReadFile(goMod)
		require.NoError(t, readErr)
		assert.Contains(t, string(result), "module github.com/org/my-service")
	})

	t.Run("replaces in YAML files", func(t *testing.T) {
		dir := t.TempDir()
		yamlFile := filepath.Join(dir, "config.yaml")
		content := `image: github.com/jrmarcello/gopherplate:latest
`
		writeErr := os.WriteFile(yamlFile, []byte(content), 0o644)
		require.NoError(t, writeErr)

		rewriteErr := RewriteModulePath(dir, "github.com/jrmarcello/gopherplate", "github.com/org/my-service")
		require.NoError(t, rewriteErr)

		result, readErr := os.ReadFile(yamlFile)
		require.NoError(t, readErr)
		assert.Contains(t, string(result), "github.com/org/my-service")
	})

	t.Run("replaces in Makefile", func(t *testing.T) {
		dir := t.TempDir()
		makefile := filepath.Join(dir, "Makefile")
		content := `MODULE = github.com/jrmarcello/gopherplate
`
		writeErr := os.WriteFile(makefile, []byte(content), 0o644)
		require.NoError(t, writeErr)

		rewriteErr := RewriteModulePath(dir, "github.com/jrmarcello/gopherplate", "github.com/org/my-service")
		require.NoError(t, rewriteErr)

		result, readErr := os.ReadFile(makefile)
		require.NoError(t, readErr)
		assert.Contains(t, string(result), "github.com/org/my-service")
	})

	t.Run("skips non-matching file types", func(t *testing.T) {
		dir := t.TempDir()
		binFile := filepath.Join(dir, "binary.dat")
		content := "github.com/jrmarcello/gopherplate"
		writeErr := os.WriteFile(binFile, []byte(content), 0o644)
		require.NoError(t, writeErr)

		rewriteErr := RewriteModulePath(dir, "github.com/jrmarcello/gopherplate", "github.com/org/my-service")
		require.NoError(t, rewriteErr)

		result, readErr := os.ReadFile(binFile)
		require.NoError(t, readErr)
		assert.Equal(t, content, string(result), "non-matching file should be untouched")
	})

	t.Run("leaves files without matches untouched", func(t *testing.T) {
		dir := t.TempDir()
		goFile := filepath.Join(dir, "other.go")
		content := `package other

func hello() {}
`
		writeErr := os.WriteFile(goFile, []byte(content), 0o644)
		require.NoError(t, writeErr)

		rewriteErr := RewriteModulePath(dir, "github.com/jrmarcello/gopherplate", "github.com/org/my-service")
		require.NoError(t, rewriteErr)

		result, readErr := os.ReadFile(goFile)
		require.NoError(t, readErr)
		assert.Equal(t, content, string(result))
	})

	t.Run("handles nested directories", func(t *testing.T) {
		dir := t.TempDir()
		nested := filepath.Join(dir, "internal", "domain")
		mkdirErr := os.MkdirAll(nested, 0o755)
		require.NoError(t, mkdirErr)

		goFile := filepath.Join(nested, "entity.go")
		content := `package domain

import "github.com/jrmarcello/gopherplate/pkg/apperror"
`
		writeErr := os.WriteFile(goFile, []byte(content), 0o644)
		require.NoError(t, writeErr)

		rewriteErr := RewriteModulePath(dir, "github.com/jrmarcello/gopherplate", "github.com/org/my-service")
		require.NoError(t, rewriteErr)

		result, readErr := os.ReadFile(goFile)
		require.NoError(t, readErr)
		assert.Contains(t, string(result), "github.com/org/my-service/pkg/apperror")
	})
}

func TestShouldRewriteFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "Go file", path: "/project/main.go", want: true},
		{name: "go.mod", path: "/project/go.mod", want: true},
		{name: "go.sum", path: "/project/go.sum", want: true},
		{name: "YAML file", path: "/project/config.yaml", want: true},
		{name: "YML file", path: "/project/config.yml", want: true},
		{name: "Makefile", path: "/project/Makefile", want: true},
		{name: "Dockerfile", path: "/project/Dockerfile", want: true},
		{name: "Markdown file", path: "/project/README.md", want: true},
		{name: "binary file", path: "/project/app.bin", want: false},
		{name: "JSON file", path: "/project/config.json", want: false},
		{name: "text file", path: "/project/notes.txt", want: false},
		{name: "image file", path: "/project/logo.png", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRewriteFile(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}
