package boilerplate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceServiceName(t *testing.T) {
	projectDir := t.TempDir()

	// Create a Makefile with the default service name.
	makefilePath := filepath.Join(projectDir, "Makefile")
	require.NoError(t, os.WriteFile(makefilePath, []byte("APP_NAME := gopherplate\nIMAGE := gopherplate-api\n"), 0o644))

	// Create a deploy file.
	deployDir := filepath.Join(projectDir, "deploy", "base")
	require.NoError(t, os.MkdirAll(deployDir, 0o755))
	deployPath := filepath.Join(deployDir, "deployment.yaml")
	require.NoError(t, os.WriteFile(deployPath, []byte("name: gopherplate\n"), 0o644))

	// Create a file that does NOT contain the service name.
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# My Project\n"), 0o644))

	replaceErr := ReplaceServiceName(projectDir, "order-service")
	require.NoError(t, replaceErr)

	// Verify Makefile was updated.
	makeContent, readErr := os.ReadFile(makefilePath)
	require.NoError(t, readErr)
	assert.Equal(t, "APP_NAME := order-service\nIMAGE := order-service-api\n", string(makeContent))

	// Verify deploy file was updated.
	deployContent, readErr := os.ReadFile(deployPath)
	require.NoError(t, readErr)
	assert.Equal(t, "name: order-service\n", string(deployContent))

	// Verify README was not modified (no match).
	readmeContent, readErr := os.ReadFile(filepath.Join(projectDir, "README.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "# My Project\n", string(readmeContent))
}

func TestReplaceServiceName_MissingFile(t *testing.T) {
	projectDir := t.TempDir()

	// No files exist. Should not error (missing files are skipped).
	replaceErr := ReplaceServiceName(projectDir, "payment-service")
	assert.NoError(t, replaceErr)
}
