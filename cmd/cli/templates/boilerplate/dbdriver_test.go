package boilerplate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwitchDBDriver_Postgres(t *testing.T) {
	projectDir := t.TempDir()

	goFile := filepath.Join(projectDir, "main.go")
	original := "package main\n\nimport (\n\t_ \"github.com/lib/pq\"\n)\n"
	require.NoError(t, os.WriteFile(goFile, []byte(original), 0o600))

	// Postgres is the default -- no changes expected.
	switchErr := SwitchDBDriver(projectDir, "postgres")
	require.NoError(t, switchErr)

	content, readErr := os.ReadFile(goFile)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(content))
}

func TestSwitchDBDriver_MySQL(t *testing.T) {
	projectDir := t.TempDir()

	goFile := filepath.Join(projectDir, "main.go")
	original := "package main\n\nimport (\n\t_ \"github.com/lib/pq\"\n)\n"
	require.NoError(t, os.WriteFile(goFile, []byte(original), 0o600))

	switchErr := SwitchDBDriver(projectDir, "mysql")
	require.NoError(t, switchErr)

	content, readErr := os.ReadFile(goFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(content), "github.com/go-sql-driver/mysql")
	assert.NotContains(t, string(content), "github.com/lib/pq")
}

func TestSwitchDBDriver_SQLite(t *testing.T) {
	projectDir := t.TempDir()

	goFile := filepath.Join(projectDir, "main.go")
	original := "package main\n\nimport (\n\t_ \"github.com/lib/pq\"\n)\n"
	require.NoError(t, os.WriteFile(goFile, []byte(original), 0o600))

	switchErr := SwitchDBDriver(projectDir, "sqlite")
	require.NoError(t, switchErr)

	content, readErr := os.ReadFile(goFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(content), "modernc.org/sqlite")
	assert.NotContains(t, string(content), "github.com/lib/pq")
}

func TestSwitchDBDriver_UnknownDriver(t *testing.T) {
	projectDir := t.TempDir()

	goFile := filepath.Join(projectDir, "main.go")
	original := "package main\n\nimport (\n\t_ \"github.com/lib/pq\"\n)\n"
	require.NoError(t, os.WriteFile(goFile, []byte(original), 0o600))

	// Unknown driver should be a no-op.
	switchErr := SwitchDBDriver(projectDir, "cockroachdb")
	require.NoError(t, switchErr)

	content, readErr := os.ReadFile(goFile)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(content))
}

func TestSwitchDBDriver_NonGoFilesUntouched(t *testing.T) {
	projectDir := t.TempDir()

	yamlFile := filepath.Join(projectDir, "config.yaml")
	yamlContent := "driver: github.com/lib/pq\n"
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0o644))

	switchErr := SwitchDBDriver(projectDir, "mysql")
	require.NoError(t, switchErr)

	content, readErr := os.ReadFile(yamlFile)
	require.NoError(t, readErr)
	assert.Equal(t, yamlContent, string(content), "non-.go files should not be modified")
}
