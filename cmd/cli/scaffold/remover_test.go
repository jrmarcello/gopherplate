package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveDisabledFeatures(t *testing.T) {
	t.Run("removes redis files when Redis is disabled", func(t *testing.T) {
		dir := t.TempDir()
		createFeatureTree(t, dir, FeatureFiles["redis"])

		cfg := DefaultConfig()
		cfg.Redis = false
		cfg.Idempotency = false // idempotency requires redis

		removeErr := RemoveDisabledFeatures(dir, cfg)
		require.NoError(t, removeErr)

		for _, rel := range FeatureFiles["redis"] {
			abs := filepath.Join(dir, rel)
			_, statErr := os.Stat(abs)
			assert.True(t, os.IsNotExist(statErr), "expected %s to be removed", rel)
		}
	})

	t.Run("removes idempotency files when Idempotency is disabled", func(t *testing.T) {
		dir := t.TempDir()
		createFeatureTree(t, dir, FeatureFiles["idempotency"])

		cfg := DefaultConfig()
		cfg.Idempotency = false

		removeErr := RemoveDisabledFeatures(dir, cfg)
		require.NoError(t, removeErr)

		for _, rel := range FeatureFiles["idempotency"] {
			abs := filepath.Join(dir, rel)
			_, statErr := os.Stat(abs)
			assert.True(t, os.IsNotExist(statErr), "expected %s to be removed", rel)
		}
	})

	t.Run("removes auth files when Auth is disabled", func(t *testing.T) {
		dir := t.TempDir()
		createFeatureTree(t, dir, FeatureFiles["auth"])

		cfg := DefaultConfig()
		cfg.Auth = false

		removeErr := RemoveDisabledFeatures(dir, cfg)
		require.NoError(t, removeErr)

		for _, rel := range FeatureFiles["auth"] {
			abs := filepath.Join(dir, rel)
			_, statErr := os.Stat(abs)
			assert.True(t, os.IsNotExist(statErr), "expected %s to be removed", rel)
		}
	})

	t.Run("removes example files when KeepExamples is disabled", func(t *testing.T) {
		dir := t.TempDir()
		createFeatureTree(t, dir, FeatureFiles["examples"])

		cfg := DefaultConfig()
		cfg.KeepExamples = false

		removeErr := RemoveDisabledFeatures(dir, cfg)
		require.NoError(t, removeErr)

		for _, rel := range FeatureFiles["examples"] {
			abs := filepath.Join(dir, rel)
			_, statErr := os.Stat(abs)
			assert.True(t, os.IsNotExist(statErr), "expected %s to be removed", rel)
		}
	})

	t.Run("keeps all files when all features are enabled", func(t *testing.T) {
		dir := t.TempDir()
		allPaths := collectAllFeaturePaths()
		createFeatureTree(t, dir, allPaths)

		cfg := DefaultConfig()
		// All features enabled by default

		removeErr := RemoveDisabledFeatures(dir, cfg)
		require.NoError(t, removeErr)

		for _, rel := range allPaths {
			abs := filepath.Join(dir, rel)
			_, statErr := os.Stat(abs)
			assert.NoError(t, statErr, "expected %s to still exist", rel)
		}
	})

	t.Run("handles already-missing paths gracefully", func(t *testing.T) {
		dir := t.TempDir()
		// Don't create any files - paths are already missing

		cfg := DefaultConfig()
		cfg.Redis = false
		cfg.Idempotency = false
		cfg.Auth = false
		cfg.KeepExamples = false

		removeErr := RemoveDisabledFeatures(dir, cfg)
		assert.NoError(t, removeErr, "should not error on missing paths")
	})
}

// createFeatureTree creates the directory structure and placeholder files
// for the given relative paths inside the root directory.
func createFeatureTree(t *testing.T, root string, paths []string) {
	t.Helper()
	for _, rel := range paths {
		abs := filepath.Join(root, rel)
		ext := filepath.Ext(rel)
		if ext != "" {
			// It's a file path - create parent dir and file
			mkdirErr := os.MkdirAll(filepath.Dir(abs), 0o750)
			require.NoError(t, mkdirErr)
			writeErr := os.WriteFile(abs, []byte("placeholder"), 0o644)
			require.NoError(t, writeErr)
		} else {
			// It's a directory path
			mkdirErr := os.MkdirAll(abs, 0o755)
			require.NoError(t, mkdirErr)
			// Create a placeholder file inside so we can verify removal
			writeErr := os.WriteFile(filepath.Join(abs, "placeholder.go"), []byte("package x"), 0o644)
			require.NoError(t, writeErr)
		}
	}
}

func collectAllFeaturePaths() []string {
	seen := make(map[string]bool)
	var result []string
	for _, paths := range FeatureFiles {
		for _, p := range paths {
			if !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}
	return result
}
