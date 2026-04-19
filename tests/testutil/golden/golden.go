// Package golden implements the approved-fixtures (a.k.a. "golden file") testing
// pattern. Tests assert that actual output matches a committed JSON fixture;
// accidental response-shape drift fails the test and forces a human review of
// the diff. Pass the `-update` flag to regenerate goldens after an intentional
// change.
//
// Typical usage in an E2E test:
//
//	golden.AssertJSONWithMask(t, "create_user_201", body,
//	    golden.Mask{Paths: []string{"id", "created_at"}})
//
// Goldens live in <package>/testdata/golden/<name>.json by default. Override
// the directory per-test by setting `golden.Dir` before calling Assert*.
package golden

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-cmp/cmp"
)

// Dir is the directory where golden files are read from and written to.
// Paths are relative to the caller's working directory (typically the package
// under test). Tests can rebind this temporarily to isolate fixtures.
var Dir = filepath.Join("testdata", "golden")

// UpdateFlag controls write-through mode. When set, Assert* overwrites the
// golden file with the current actual value instead of comparing. Exposed as
// a pointer so tests can toggle it; production usage is `go test ... -update`.
var UpdateFlag = flag.Bool("update", false, "overwrite golden files with current actual values")

// sentinel is substituted for masked values before diffing. Using a distinct
// string (not just "") makes the masked key visible in diffs when the mask
// mismatches expectations.
const sentinel = "<masked>"

// Mask declares JSON paths whose values are replaced by a sentinel before the
// golden comparison. Supports dotted nested keys like "user.id". Missing
// paths are silently ignored (no-op), so callers can safely mask optional
// fields.
type Mask struct {
	Paths []string
}

// T is the minimal testing surface Assert* depends on. Both *testing.T and
// custom shims (for internal tests) satisfy it.
type T interface {
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Helper()
}

// AssertJSON compares actualBody against the committed golden file named
// <Dir>/<name>.json. Calls t.Errorf with a human-readable diff on mismatch.
// In update mode, rewrites the golden file and does not compare.
func AssertJSON(t T, name string, actualBody []byte) {
	t.Helper()
	assertJSON(t, name, actualBody, Mask{})
}

// AssertJSONWithMask is like AssertJSON but zeros the given paths in both the
// golden and the actual body before diffing. Use for fields that legitimately
// change between runs (UUIDs, timestamps) but whose presence/type still
// matters.
func AssertJSONWithMask(t T, name string, actualBody []byte, mask Mask) {
	t.Helper()
	assertJSON(t, name, actualBody, mask)
}

func assertJSON(t T, name string, actualBody []byte, mask Mask) {
	t.Helper()

	path := filepath.Join(Dir, name+".json")

	if *UpdateFlag {
		if writeErr := writeGolden(path, actualBody); writeErr != nil {
			t.Fatalf("golden: writing %s: %v", path, writeErr)
		}
		return
	}

	var actual any
	if unmarshalErr := json.Unmarshal(actualBody, &actual); unmarshalErr != nil {
		t.Errorf("golden: actual body is not valid JSON: %v\n  body: %s",
			unmarshalErr, truncate(string(actualBody), 200))
		return
	}

	goldenBytes, readErr := os.ReadFile(path) //nolint:gosec // G304: path is test-controlled
	if readErr != nil {
		if os.IsNotExist(readErr) {
			t.Errorf("golden: fixture not found at %s — run `go test ... -update` to create it\n  actual: %s",
				path, string(actualBody))
			return
		}
		t.Fatalf("golden: reading %s: %v", path, readErr)
		return
	}

	var expected any
	if unmarshalErr := json.Unmarshal(goldenBytes, &expected); unmarshalErr != nil {
		t.Fatalf("golden: fixture %s is malformed JSON: %v", path, unmarshalErr)
		return
	}

	applyMask(expected, mask)
	applyMask(actual, mask)

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("golden: %s mismatch (-want +got):\n%s", path, diff)
	}
}

// writeGolden pretty-prints the JSON body and writes it to path. Stable
// formatting keeps diffs of the golden file itself readable on review.
func writeGolden(path string, body []byte) error {
	var decoded any
	if unmarshalErr := json.Unmarshal(body, &decoded); unmarshalErr != nil {
		return fmt.Errorf("input is not valid JSON: %w", unmarshalErr)
	}
	pretty, marshalErr := json.MarshalIndent(decoded, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("marshaling: %w", marshalErr)
	}
	pretty = append(pretty, '\n')

	if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o750); mkdirErr != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), mkdirErr)
	}
	if writeErr := os.WriteFile(path, pretty, 0o600); writeErr != nil {
		return fmt.Errorf("write %s: %w", path, writeErr)
	}
	return nil
}

// applyMask walks the decoded JSON tree and replaces each masked path's value
// with the sentinel. Non-existent paths are no-ops. Supports dotted keys only;
// array indexing / wildcards are a future extension.
func applyMask(tree any, mask Mask) {
	for _, path := range mask.Paths {
		applyOne(tree, strings.Split(path, "."))
	}
}

func applyOne(tree any, segments []string) {
	if len(segments) == 0 {
		return
	}
	m, ok := tree.(map[string]any)
	if !ok {
		return
	}
	key := segments[0]
	if len(segments) == 1 {
		if _, exists := m[key]; exists {
			m[key] = sentinel
		}
		return
	}
	child, exists := m[key]
	if !exists {
		return
	}
	applyOne(child, segments[1:])
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
