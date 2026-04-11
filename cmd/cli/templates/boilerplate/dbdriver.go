package boilerplate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DBDriverConfig holds the import path and driver name for a database driver.
type DBDriverConfig struct {
	ImportPath string // e.g. "github.com/lib/pq"
	DriverName string // e.g. "postgres"
}

// DBDrivers maps supported database choices to their driver configuration.
var DBDrivers = map[string]DBDriverConfig{
	"postgres": {ImportPath: "github.com/lib/pq", DriverName: "postgres"},
	"mysql":    {ImportPath: "github.com/go-sql-driver/mysql", DriverName: "mysql"},
	"sqlite":   {ImportPath: "modernc.org/sqlite", DriverName: "sqlite"},
}

const defaultDriverImport = "github.com/lib/pq"

// SwitchDBDriver replaces the default postgres driver import with the chosen
// driver in all Go files under projectDir. If dbChoice is "postgres" or
// unrecognized, no changes are made.
func SwitchDBDriver(projectDir, dbChoice string) error {
	driver, ok := DBDrivers[dbChoice]
	if !ok || dbChoice == "postgres" {
		return nil // default driver, nothing to change
	}

	return filepath.Walk(projectDir, func(path string, info os.FileInfo, walkErr error) error { //nolint:gosec // CLI scaffold tool, TOCTOU risk is acceptable
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		content, readErr := os.ReadFile(path) //nolint:gosec // CLI tool reads project files for driver substitution
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", path, readErr)
		}

		oldContent := string(content)
		newContent := strings.ReplaceAll(oldContent, defaultDriverImport, driver.ImportPath)

		if oldContent == newContent {
			return nil
		}

		writeErr := os.WriteFile(path, []byte(newContent), info.Mode()) //nolint:gosec // CLI tool writes to user-specified project directory
		if writeErr != nil {
			return fmt.Errorf("writing %s: %w", path, writeErr)
		}
		return nil
	})
}
