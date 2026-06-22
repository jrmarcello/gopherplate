package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TC-11: isStale — newer/equal/older
func TestIsStale(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.True(t, isStale(base, base.Add(time.Hour)), "newer commit → stale")
	assert.False(t, isStale(base, base), "equal → not stale")
	assert.False(t, isStale(base, base.Add(-time.Hour)), "older commit → not stale")
}

// TC-08/09 (unit): parseCapabilityDoc extracts the PATH from a "<path> — <description>" bullet
// (and from a backtick-wrapped path), NOT the whole line. Regression guard for the
// description-suffix format the real capability docs use.
func TestParseCapabilityDoc_ExtractsPathFromDescribedBullet(t *testing.T) {
	t.Parallel()
	content := "## Code\n\n" +
		"- pkg/cache/ — Cache interface\n" +
		"- `internal/usecases/user/` — CRUD use cases\n\n" +
		"Last-verified: 2026-01-01 (abc1234)\n\n" +
		"## Guarantees (current truth)\n"
	doc := parseCapabilityDoc("sample", content)
	assert.True(t, doc.hasCodeSec, "## Code section detected")
	assert.Equal(t, []string{"pkg/cache/", "internal/usecases/user/"}, doc.codePaths,
		"path must be the first field, backticks stripped, the em-dash description dropped")
	assert.Equal(t, 2026, doc.lastVerified.Year(), "Last-verified date parsed")
}

// TC-08: dead ## Code path → ERROR naming doc + path
func TestRunCapabilities_DeadPath_Error(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "caps-deadpath")
	code := runCapabilities(dir, func(absPath string) (time.Time, bool) {
		return time.Time{}, false
	})
	assert.Equal(t, 1, code, "dead code path → exit 1")
}

// TC-09: all ## Code paths exist → no error
func TestRunCapabilities_Valid_NoError(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "caps-valid")
	code := runCapabilities(dir, func(absPath string) (time.Time, bool) {
		return time.Time{}, false
	})
	assert.Equal(t, 0, code, "valid code paths → exit 0")
}

// TC-10: no ## Code section → WARN (exit 0)
func TestRunCapabilities_NoCode_Warn(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "caps-nocode")
	code := runCapabilities(dir, func(absPath string) (time.Time, bool) {
		return time.Time{}, false
	})
	assert.Equal(t, 0, code, "no ## Code section → exit 0 (warn only)")
}

// TC-12: capabilities exit codes
func TestRunCapabilities_ExitCodes(t *testing.T) {
	t.Parallel()
	// dead-path → 1
	assert.Equal(t, 1, runCapabilities(filepath.Join("testdata", "caps-deadpath"), func(absPath string) (time.Time, bool) {
		return time.Time{}, false
	}))
	// all-valid → 0
	assert.Equal(t, 0, runCapabilities(filepath.Join("testdata", "caps-valid"), func(absPath string) (time.Time, bool) {
		return time.Time{}, false
	}))
	// no-##Code → 0
	assert.Equal(t, 0, runCapabilities(filepath.Join("testdata", "caps-nocode"), func(absPath string) (time.Time, bool) {
		return time.Time{}, false
	}))
}

// TC-11 stale: stale doc + injected newer commit → WARN (exit 0)
func TestRunCapabilities_Stale_Warn(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "caps-stale")
	// Inject: commit date is after 2020-01-01 (last-verified date in fixture)
	later := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	code := runCapabilities(dir, func(absPath string) (time.Time, bool) {
		return later, true
	})
	assert.Equal(t, 0, code, "stale doc with newer commit → warn only → exit 0")
}

// TC-18: injected commit-date lookup returns (zero, false) → no staleness WARN
func TestRunCapabilities_NoStalenessWhenLookupFails(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "caps-stale")
	code := runCapabilities(dir, func(absPath string) (time.Time, bool) {
		return time.Time{}, false // simulate git absent
	})
	assert.Equal(t, 0, code, "when commit lookup returns false → no warn → exit 0")
}

// TC-21: capabilities over dir with MANIFEST.md+README.md+valid doc → findings only for valid doc
func TestRunCapabilities_SkipMetaDocs(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "caps-skip")
	code := runCapabilities(dir, func(absPath string) (time.Time, bool) {
		return time.Time{}, false
	})
	assert.Equal(t, 0, code, "MANIFEST.md and README.md must be skipped; valid doc → exit 0")
}
