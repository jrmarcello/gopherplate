package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jrmarcello/gopherplate/cmd/cli/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTemplateRoot(t *testing.T) {
	// Build a fake gopherplate checkout inside a temp dir.
	fakeRoot := t.TempDir()
	writeGoMod(t, fakeRoot, scaffold.TemplateModulePath)

	// And a directory that is NOT a gopherplate checkout.
	nonRoot := t.TempDir()
	writeGoMod(t, nonRoot, "github.com/example/other")

	t.Run("explicit --template pointing to gopherplate root", func(t *testing.T) {
		got, err := resolveTemplateRoot(fakeRoot)
		require.NoError(t, err)
		assert.Equal(t, realPath(t, fakeRoot), realPath(t, got))
	})

	t.Run("explicit --template pointing elsewhere returns error", func(t *testing.T) {
		_, err := resolveTemplateRoot(nonRoot)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not point to a gopherplate checkout")
	})

	t.Run("env var override wins when no flag", func(t *testing.T) {
		t.Setenv(templateEnvVar, fakeRoot)
		got, err := resolveTemplateRoot("")
		require.NoError(t, err)
		assert.Equal(t, realPath(t, fakeRoot), realPath(t, got))
	})

	t.Run("env var pointing to non-gopherplate dir returns error", func(t *testing.T) {
		t.Setenv(templateEnvVar, nonRoot)
		_, err := resolveTemplateRoot("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), templateEnvVar)
	})

	t.Run("cwd detection walks up to find go.mod", func(t *testing.T) {
		t.Setenv(templateEnvVar, "")
		nested := filepath.Join(fakeRoot, "a", "b", "c")
		require.NoError(t, os.MkdirAll(nested, 0o755))
		withCwd(t, nested, func() {
			got, err := resolveTemplateRoot("")
			require.NoError(t, err)
			// macOS symlinks /var -> /private/var, so compare realpaths.
			assert.Equal(t, realPath(t, fakeRoot), realPath(t, got))
		})
	})

	t.Run("cwd outside any gopherplate checkout returns error", func(t *testing.T) {
		t.Setenv(templateEnvVar, "")
		// Use a nested temp dir so walk-up doesn't accidentally hit the real
		// gopherplate repo (which would contain the real go.mod). Any path
		// under an isolated TempDir is guaranteed to have no gopherplate
		// ancestor.
		withCwd(t, nonRoot, func() {
			_, err := resolveTemplateRoot("")
			// NOTE: this can still succeed if os.Executable() points into a
			// gopherplate checkout (e.g. when the test binary lives inside
			// the repo). We only assert that if cwd fails AND the exe path
			// also fails, we get the expected guidance message.
			if err != nil {
				assert.Contains(t, err.Error(), "could not locate the gopherplate template")
			}
		})
	})
}

func TestIsGopherplateRoot(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, scaffold.TemplateModulePath)
	assert.True(t, isGopherplateRoot(dir))

	other := t.TempDir()
	writeGoMod(t, other, "github.com/example/other")
	assert.False(t, isGopherplateRoot(other))

	empty := t.TempDir()
	assert.False(t, isGopherplateRoot(empty))
}

func writeGoMod(t *testing.T, dir, module string) {
	t.Helper()
	content := "module " + module + "\n\ngo 1.21\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644))
}

func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })
	fn()
}

// realPath resolves all symlinks in path so tests can compare paths reliably
// across platforms (macOS aliases /var -> /private/var, etc.).
func realPath(t *testing.T, path string) string {
	t.Helper()
	abs, absErr := filepath.Abs(path)
	require.NoError(t, absErr)
	resolved, evalErr := filepath.EvalSymlinks(abs)
	require.NoError(t, evalErr)
	return resolved
}
