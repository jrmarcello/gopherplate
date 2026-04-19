package flavors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tempRoot returns a test-scoped scaffold root with optional seed files.
func tempRoot(t *testing.T, seed map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range seed {
		full := filepath.Join(root, rel)
		if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
			t.Fatalf("mkdir %s: %v", full, mkErr)
		}
		if writeErr := os.WriteFile(full, []byte(content), 0o600); writeErr != nil {
			t.Fatalf("write %s: %v", full, writeErr)
		}
	}
	return root
}

func readRel(t *testing.T, root, rel string) string {
	t.Helper()
	b, readErr := os.ReadFile(filepath.Join(root, rel)) //nolint:gosec // test scope
	if readErr != nil {
		t.Fatalf("read %s: %v", rel, readErr)
	}
	return string(b)
}

func TestApply_Create(t *testing.T) {
	root := tempRoot(t, nil)
	applier := NewApplier(root)

	o := Overlay{Action: ActionCreate, Path: "new/file.txt", Template: "hello world"}
	if err := applier.Apply(o, nil); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := readRel(t, root, "new/file.txt"); got != "hello world" {
		t.Errorf("content = %q, want %q", got, "hello world")
	}

	t.Run("TC-UC-11 create on existing path returns conflict error", func(t *testing.T) {
		dup := Overlay{Action: ActionCreate, Path: "new/file.txt", Template: "other"}
		err := applier.Apply(dup, nil)
		if err == nil {
			t.Fatalf("expected error on create-over-existing")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error should say 'already exists', got: %v", err)
		}
	})
}

func TestApply_Append_TC_UC_04(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"Makefile": "base: ## base target\n\techo base\n",
	})
	applier := NewApplier(root)

	o := Overlay{
		Action:   ActionAppend,
		Path:     "Makefile",
		Template: "\nmutation: ## mutation test\n\techo mut\n",
	}
	if err := applier.Apply(o, nil); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	got := readRel(t, root, "Makefile")
	if !strings.Contains(got, "base:") {
		t.Errorf("base target missing after append: %s", got)
	}
	if !strings.Contains(got, "mutation:") {
		t.Errorf("flavor target missing after append: %s", got)
	}
}

func TestApply_InsertMarker_TC_UC_10(t *testing.T) {
	t.Run("happy: marker present → inserted below", func(t *testing.T) {
		root := tempRoot(t, map[string]string{
			"server.go": "package main\n\n// @flavor-di-wiring\n\nfunc main() {}\n",
		})
		applier := NewApplier(root)
		o := Overlay{
			Action:   ActionInsertMarker,
			Path:     "server.go",
			Marker:   "// @flavor-di-wiring",
			Template: "var extra = 1\n",
		}
		if err := applier.Apply(o, nil); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		got := readRel(t, root, "server.go")
		if !strings.Contains(got, "@flavor-di-wiring") {
			t.Errorf("marker line should remain, got: %s", got)
		}
		if !strings.Contains(got, "var extra = 1") {
			t.Errorf("template not inserted, got: %s", got)
		}
	})

	t.Run("TC-UC-10 marker absent → error with marker name", func(t *testing.T) {
		root := tempRoot(t, map[string]string{
			"server.go": "package main\n\nfunc main() {}\n",
		})
		applier := NewApplier(root)
		o := Overlay{
			Action:   ActionInsertMarker,
			Path:     "server.go",
			Marker:   "// @flavor-missing-marker",
			Template: "injected",
		}
		err := applier.Apply(o, nil)
		if err == nil {
			t.Fatalf("expected error when marker absent")
		}
		if !strings.Contains(err.Error(), "@flavor-missing-marker") {
			t.Errorf("error should name the missing marker, got: %v", err)
		}
	})
}

func TestApply_Overwrite_TC_UC_05_UC_06(t *testing.T) {
	t.Run("TC-UC-05 overwrite with // overlay: overwrite sentinel → replaces", func(t *testing.T) {
		root := tempRoot(t, map[string]string{
			"config.yml": "base: yes\n",
		})
		applier := NewApplier(root)
		// Overlay must declare overwrite explicitly and the Template must
		// start with a sentinel comment so we distinguish accidental
		// overwrites from intentional ones (defense in depth).
		o := Overlay{
			Action:   ActionOverwrite,
			Path:     "config.yml",
			Template: "# overlay: overwrite justified-by-flavor\nflavor: yes\n",
		}
		warnings, err := applier.ApplyWithWarnings(o, nil)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(warnings) == 0 {
			t.Errorf("expected warning for explicit overwrite")
		}
		got := readRel(t, root, "config.yml")
		if strings.Contains(got, "base: yes") {
			t.Errorf("base should have been replaced, got: %s", got)
		}
		if !strings.Contains(got, "flavor: yes") {
			t.Errorf("flavor content missing, got: %s", got)
		}
	})

	t.Run("TC-UC-06 overwrite without sentinel comment → conflict error", func(t *testing.T) {
		root := tempRoot(t, map[string]string{
			"config.yml": "base: yes\n",
		})
		applier := NewApplier(root)
		o := Overlay{
			Action:   ActionOverwrite,
			Path:     "config.yml",
			Template: "flavor: yes\n", // missing sentinel
		}
		err := applier.Apply(o, nil)
		if err == nil {
			t.Fatalf("expected error without sentinel")
		}
		if !strings.Contains(err.Error(), "overwrite") {
			t.Errorf("error should mention overwrite, got: %v", err)
		}
	})
}

func TestApply_Template_TC_UC_12(t *testing.T) {
	root := tempRoot(t, nil)
	applier := NewApplier(root)

	t.Run("happy: template renders with data", func(t *testing.T) {
		o := Overlay{
			Action:   ActionCreate,
			Path:     "greet.txt",
			Template: "hello {{.Name}}",
		}
		if err := applier.Apply(o, map[string]string{"Name": "alice"}); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if got := readRel(t, root, "greet.txt"); got != "hello alice" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("TC-UC-12 invalid template syntax → render error", func(t *testing.T) {
		o := Overlay{
			Action:   ActionCreate,
			Path:     "bad.txt",
			Template: "hello {{.Name", // unclosed action
		}
		err := applier.Apply(o, map[string]string{"Name": "x"})
		if err == nil {
			t.Fatalf("expected template parse error")
		}
	})
}

func TestApply_PathTraversal_TC_UC_14(t *testing.T) {
	root := tempRoot(t, nil)
	applier := NewApplier(root)

	cases := []string{
		"../../../etc/passwd",
		"..",
		"subdir/../../outside.txt",
	}
	for _, p := range cases {
		t.Run("reject "+p, func(t *testing.T) {
			o := Overlay{Action: ActionCreate, Path: p, Template: "x"}
			err := applier.Apply(o, nil)
			if err == nil {
				t.Fatalf("expected path-escape error for %q", p)
			}
			if !strings.Contains(err.Error(), "escape") && !strings.Contains(err.Error(), "invalid path") {
				t.Errorf("error should mention path escape, got: %v", err)
			}
		})
	}
}

func TestApply_GoModRequire_TC_UC_13(t *testing.T) {
	initialMod := `module github.com/example/demo

go 1.22

require (
	github.com/redis/go-redis/v9 v9.10.0
)
`
	t.Run("happy: new require added", func(t *testing.T) {
		root := tempRoot(t, map[string]string{"go.mod": initialMod})
		applier := NewApplier(root)
		o := Overlay{
			Action: ActionGoModRequire,
			Path:   "go.mod",
			Module: "github.com/google/go-cmp v0.7.0",
		}
		if err := applier.Apply(o, nil); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		got := readRel(t, root, "go.mod")
		if !strings.Contains(got, "github.com/google/go-cmp v0.7.0") {
			t.Errorf("new require not added, got: %s", got)
		}
		if !strings.Contains(got, "github.com/redis/go-redis/v9 v9.10.0") {
			t.Errorf("existing require removed, got: %s", got)
		}
	})

	t.Run("TC-UC-13 conflicting version: max wins + warning", func(t *testing.T) {
		root := tempRoot(t, map[string]string{"go.mod": initialMod})
		applier := NewApplier(root)
		o := Overlay{
			Action: ActionGoModRequire,
			Path:   "go.mod",
			Module: "github.com/redis/go-redis/v9 v9.11.0", // higher
		}
		warnings, err := applier.ApplyWithWarnings(o, nil)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if len(warnings) == 0 {
			t.Errorf("expected warning on version bump")
		}
		got := readRel(t, root, "go.mod")
		if !strings.Contains(got, "v9.11.0") {
			t.Errorf("higher version should win, got: %s", got)
		}
		if strings.Contains(got, "v9.10.0") {
			t.Errorf("lower version should be replaced, got: %s", got)
		}
	})

	t.Run("lower version requested → keep existing (max wins) + warning", func(t *testing.T) {
		root := tempRoot(t, map[string]string{"go.mod": initialMod})
		applier := NewApplier(root)
		o := Overlay{
			Action: ActionGoModRequire,
			Path:   "go.mod",
			Module: "github.com/redis/go-redis/v9 v9.5.0", // lower
		}
		warnings, err := applier.ApplyWithWarnings(o, nil)
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if len(warnings) == 0 {
			t.Errorf("expected warning when keeping existing higher version")
		}
		got := readRel(t, root, "go.mod")
		if !strings.Contains(got, "v9.10.0") {
			t.Errorf("higher existing version should remain, got: %s", got)
		}
	})
}
