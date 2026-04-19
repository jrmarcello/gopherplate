package golden_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrmarcello/gopherplate/tests/testutil/golden"
)

// withTempdirGolden swaps the package's Dir for a test-scoped directory and
// restores it on cleanup. Lets each test work in isolation without touching
// committed fixtures.
func withTempdirGolden(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := golden.Dir
	golden.Dir = dir
	t.Cleanup(func() { golden.Dir = orig })
	return dir
}

// fakeT captures t.Errorf / t.Fatalf without failing the outer test. Lets us
// assert that AssertJSON correctly reports mismatches.
type fakeT struct {
	testing.TB
	failed  bool
	fatal   bool
	message string
}

func (f *fakeT) Errorf(format string, args ...any) {
	f.failed = true
	f.message = fmtSprint(format, args)
}
func (f *fakeT) Fatalf(format string, args ...any) {
	f.fatal = true
	f.failed = true
	f.message = fmtSprint(format, args)
}
func (f *fakeT) Helper() {}

func fmtSprint(format string, args []any) string {
	var b strings.Builder
	_, _ = b.WriteString(format)
	for _, a := range args {
		_, _ = b.WriteString(" ")
		switch v := a.(type) {
		case string:
			b.WriteString(v)
		case error:
			b.WriteString(v.Error())
		}
	}
	return b.String()
}

func TestAssertJSON(t *testing.T) {
	t.Run("TC-UC-01 existing golden matches actual", func(t *testing.T) {
		dir := withTempdirGolden(t)
		writeFile(t, filepath.Join(dir, "ok.json"), `{"name":"alice","age":30}`)

		ft := &fakeT{TB: t}
		golden.AssertJSON(ft, "ok", []byte(`{"name":"alice","age":30}`))

		if ft.failed {
			t.Errorf("expected no failure, got: %s", ft.message)
		}
	})

	t.Run("TC-UC-02 golden differs from actual -> fail with diff", func(t *testing.T) {
		dir := withTempdirGolden(t)
		writeFile(t, filepath.Join(dir, "diff.json"), `{"name":"alice"}`)

		ft := &fakeT{TB: t}
		golden.AssertJSON(ft, "diff", []byte(`{"name":"bob"}`))

		if !ft.failed {
			t.Errorf("expected failure, got none")
		}
		if !strings.Contains(ft.message, "bob") && !strings.Contains(ft.message, "alice") {
			t.Errorf("expected diff to mention values, got: %s", ft.message)
		}
	})

	t.Run("TC-UC-03 -update flag overwrites golden", func(t *testing.T) {
		dir := withTempdirGolden(t)
		// Existing golden with stale content.
		writeFile(t, filepath.Join(dir, "upd.json"), `{"stale":true}`)

		// Simulate -update flag set.
		origUpdate := *golden.UpdateFlag
		*golden.UpdateFlag = true
		t.Cleanup(func() { *golden.UpdateFlag = origUpdate })

		ft := &fakeT{TB: t}
		golden.AssertJSON(ft, "upd", []byte(`{"fresh":true}`))

		if ft.failed {
			t.Errorf("expected no failure in update mode, got: %s", ft.message)
		}
		got := readFile(t, filepath.Join(dir, "upd.json"))
		// Golden is pretty-printed, so match on key+value tolerantly.
		if !strings.Contains(got, `"fresh"`) || !strings.Contains(got, `true`) {
			t.Errorf("golden file was not updated, content: %s", got)
		}
		if strings.Contains(got, `"stale"`) {
			t.Errorf("stale content still present: %s", got)
		}
	})

	t.Run("TC-UC-06 golden missing without -update -> fail with clear message", func(t *testing.T) {
		_ = withTempdirGolden(t)

		ft := &fakeT{TB: t}
		golden.AssertJSON(ft, "doesnotexist", []byte(`{}`))

		if !ft.failed {
			t.Errorf("expected failure for missing golden")
		}
		if !strings.Contains(ft.message, "doesnotexist") && !strings.Contains(ft.message, "not found") &&
			!strings.Contains(ft.message, "no such") && !strings.Contains(ft.message, "missing") {
			t.Errorf("expected clear message mentioning the missing golden, got: %s", ft.message)
		}
	})

	t.Run("TC-UC-07 invalid JSON input -> fail with clear message", func(t *testing.T) {
		dir := withTempdirGolden(t)
		writeFile(t, filepath.Join(dir, "bad.json"), `{"ok":true}`)

		ft := &fakeT{TB: t}
		golden.AssertJSON(ft, "bad", []byte(`{not json`))

		if !ft.failed {
			t.Errorf("expected failure for invalid JSON input")
		}
	})
}

func TestAssertJSONWithMask(t *testing.T) {
	t.Run("TC-UC-04 mask on id with UUID value passes when only id differs", func(t *testing.T) {
		dir := withTempdirGolden(t)
		writeFile(t, filepath.Join(dir, "mask.json"),
			`{"id":"019da17d-3498-7265-bb68-472183d6b857","name":"alice"}`)

		ft := &fakeT{TB: t}
		golden.AssertJSONWithMask(ft, "mask",
			[]byte(`{"id":"019ddddd-eeee-4aaa-bbbb-ffffffffffff","name":"alice"}`),
			golden.Mask{Paths: []string{"id"}})

		if ft.failed {
			t.Errorf("expected match after masking id, got: %s", ft.message)
		}
	})

	t.Run("TC-UC-04 (unmasked) change in id still fails without mask", func(t *testing.T) {
		dir := withTempdirGolden(t)
		writeFile(t, filepath.Join(dir, "nomask.json"),
			`{"id":"a","name":"alice"}`)

		ft := &fakeT{TB: t}
		golden.AssertJSON(ft, "nomask",
			[]byte(`{"id":"b","name":"alice"}`))

		if !ft.failed {
			t.Errorf("expected failure without mask on changed id")
		}
	})

	t.Run("TC-UC-05 mask on absent field is a no-op (still passes)", func(t *testing.T) {
		dir := withTempdirGolden(t)
		writeFile(t, filepath.Join(dir, "absent.json"),
			`{"name":"alice"}`)

		ft := &fakeT{TB: t}
		golden.AssertJSONWithMask(ft, "absent",
			[]byte(`{"name":"alice"}`),
			golden.Mask{Paths: []string{"missing_field"}})

		if ft.failed {
			t.Errorf("mask on absent field should be a no-op, got: %s", ft.message)
		}
	})

	t.Run("nested path masking: user.id", func(t *testing.T) {
		dir := withTempdirGolden(t)
		writeFile(t, filepath.Join(dir, "nested.json"),
			`{"user":{"id":"a","name":"alice"}}`)

		ft := &fakeT{TB: t}
		golden.AssertJSONWithMask(ft, "nested",
			[]byte(`{"user":{"id":"b","name":"alice"}}`),
			golden.Mask{Paths: []string{"user.id"}})

		if ft.failed {
			t.Errorf("expected match with nested mask, got: %s", ft.message)
		}
	})
}

// Helpers -------------------------------------------------------------------

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // G304 in tests: path is constructed from t.TempDir()
	if err != nil {
		t.Fatalf("readFile: %v", err)
	}
	return string(b)
}

// Ensure flag registration doesn't interfere with `go test` flag parsing.
var _ = flag.CommandLine
