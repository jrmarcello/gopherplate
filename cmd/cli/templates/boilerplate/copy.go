package boilerplate

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CopyProject copies the template project from srcDir to dstDir,
// skipping all paths listed in ExcludePaths.
func CopyProject(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walking %s: %w", path, walkErr)
		}

		relPath, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, relErr)
		}

		// Root directory is always processed.
		if relPath == "." {
			return nil
		}

		if shouldExclude(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		targetPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			mkdirErr := os.MkdirAll(targetPath, 0o750) //nolint:gosec // CLI scaffold creates project directories
			if mkdirErr != nil {
				return fmt.Errorf("creating directory %s: %w", targetPath, mkdirErr)
			}
			return nil
		}

		return copyFile(path, targetPath, info.Mode())
	})
}

// shouldExclude returns true if relPath matches any entry in ExcludePaths.
// Directory entries (ending with "/") match any path with that prefix.
// File entries match the exact relative path.
func shouldExclude(relPath string) bool {
	// Normalize to forward slashes for consistent matching.
	normalized := filepath.ToSlash(relPath)

	for _, excl := range ExcludePaths {
		if strings.HasSuffix(excl, "/") {
			// Directory prefix match.
			if strings.HasPrefix(normalized, excl) || normalized+"/" == excl {
				return true
			}
		} else {
			// Exact file match.
			if normalized == excl {
				return true
			}
		}
	}
	return false
}

// copyFile copies a single file preserving the given mode.
func copyFile(src, dst string, mode fs.FileMode) error {
	if mkdirErr := os.MkdirAll(filepath.Dir(dst), 0o750); mkdirErr != nil { //nolint:gosec // CLI scaffold creates project directories
		return fmt.Errorf("creating parent directory for %s: %w", dst, mkdirErr)
	}

	srcFile, openErr := os.Open(src) //nolint:gosec // CLI tool copies user-specified project files
	if openErr != nil {
		return fmt.Errorf("opening source %s: %w", src, openErr)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, createErr := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode) //nolint:gosec // CLI tool writes to user-specified project directory
	if createErr != nil {
		return fmt.Errorf("creating destination %s: %w", dst, createErr)
	}
	defer func() { _ = dstFile.Close() }()

	if _, copyErr := io.Copy(dstFile, srcFile); copyErr != nil {
		return fmt.Errorf("copying %s to %s: %w", src, dst, copyErr)
	}
	return nil
}
