package scaffold

import (
	"os"
	"path/filepath"
	"strings"
)

// RewriteModulePath replaces all occurrences of oldModule with newModule
// in all .go files and go.mod within the given directory tree.
func RewriteModulePath(dir, oldModule, newModule string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error { //nolint:gosec // CLI scaffold tool, TOCTOU risk is acceptable
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		// Only process relevant files
		if !shouldRewriteFile(path) {
			return nil
		}

		content, readErr := os.ReadFile(path) //nolint:gosec // CLI tool reads user-specified paths
		if readErr != nil {
			return readErr
		}

		oldContent := string(content)
		newContent := strings.ReplaceAll(oldContent, oldModule, newModule)

		if oldContent == newContent {
			return nil
		}

		return os.WriteFile(path, []byte(newContent), info.Mode()) //nolint:gosec // CLI scaffold tool, TOCTOU risk is acceptable
	})
}

func shouldRewriteFile(path string) bool {
	ext := filepath.Ext(path)
	base := filepath.Base(path)

	switch {
	case ext == ".go":
		return true
	case base == "go.mod":
		return true
	case base == "go.sum":
		return true
	case ext == ".yaml" || ext == ".yml":
		return true
	case base == "Makefile":
		return true
	case base == "Dockerfile":
		return true
	case ext == ".md":
		return true
	default:
		return false
	}
}
