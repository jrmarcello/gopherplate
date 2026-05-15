// cli_contract_test.go implements the refactor-specific contract tests
// (TC-CT-01..TC-CT-12) for the flat-layout `package learn`. Each test invokes
// NewRootCmd() + RegisterAll() + RunCmd() IN-PROCESS — no subprocess spawn —
// to verify that the post-flatten CLI surface is identical to the baseline
// captured during TASK-BASELINE:
//
//   - 16 canonical subcommands wired via init() in sibling files;
//   - root exit codes (0 ok / 1 usage / 2 runtime) preserved across refactor;
//   - persistent flag inheritance unchanged;
//   - structured slog stderr emission still parseable as JSON.
//
// These tests are derived from TASK-8 of
// `.specs/refactor-tools-learn-flat-layout.md`.
package learn

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// canonicalSubcommands lists every subcommand the `learn` binary must expose
// after the flat-layout refactor. The list is the source of truth for TC-CT-01,
// TC-CT-02, and TC-CT-06 — changes here imply REQ shifts in
// refactor-tools-learn-flat-layout.md.
var canonicalSubcommands = []string{
	"apply-decision",
	"audit-skills-prep",
	"complete-task",
	"extract",
	"init",
	"nudge-tick",
	"recall",
	"record-decision",
	"refine-apply",
	"reindex",
	"rollback",
	"similar",
	"smoke",
	"stats",
	"track-use",
	"validate-skill",
}

// buildRoot constructs a fresh root with all subcommands attached. Each test
// builds its own root to avoid leaking cobra state across tests.
func buildRoot(t *testing.T) *cobra.Command {
	t.Helper()
	root := NewRootCmd()
	RegisterAll(root)
	return root
}

// TC-CT-01: `learn --help` lists every canonical subcommand in stdout.
func Test_TC_CT_01_help_lists_all_subcommands(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	root := buildRoot(t)
	code := RunCmd(root, []string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0 for --help, got %d (stderr=%q)", code, stderr.String())
	}
	out := stdout.String()
	for _, name := range canonicalSubcommands {
		if !strings.Contains(out, name) {
			t.Errorf("--help output missing subcommand %q; got:\n%s", name, out)
		}
	}
}

// TC-CT-02: every canonical subcommand's `--help` exits 0 with non-empty
// stdout AND empty stderr (per parent-spec TC-CT-02). A subcommand that
// emits a startup warning or stray log line to stderr while reporting
// --help would silently degrade the operator UX and is a regression we
// want this TC to surface.
func Test_TC_CT_02_subcommand_help_exits_zero(t *testing.T) {
	t.Parallel()
	for _, name := range canonicalSubcommands {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			root := buildRoot(t)
			code := RunCmd(root, []string{name, "--help"}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("%s --help: expected exit 0, got %d (stderr=%q)", name, code, stderr.String())
			}
			if strings.TrimSpace(stdout.String()) == "" {
				t.Errorf("%s --help: expected non-empty stdout", name)
			}
			if s := strings.TrimSpace(stderr.String()); s != "" {
				t.Errorf("%s --help: expected empty stderr, got %q", name, s)
			}
		})
	}
}

// TC-CT-03: unknown subcommand yields a usage error (exit 1).
func Test_TC_CT_03_unknown_subcommand_exits_one(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	root := buildRoot(t)
	code := RunCmd(root, []string{"unknownsubcmd"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for unknown subcommand, got %d (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Errorf("expected stderr to contain \"unknown command\", got: %q", stderr.String())
	}
}

// TC-CT-04: runtime error path exits 2 AND emits a level=ERROR slog record
// whose JSON envelope describes the failure cause. `stats` against an
// unreachable DB parent directory triggers cmd_stats.go's structured
// slog.Error() call BEFORE the *runtimeError bubbles up to RunCmd's
// fmt.Fprintln. We use `stats` here (not `recall`) because cmd_recall.go's
// runtime-error path emits no slog record — only RunCmd's raw stderr
// fallback — and exercising the slog wiring is the load-bearing
// assertion this TC owns.
//
// Three load-bearing assertions per parent-spec TC-CT-04 / fix-spec REQ-2:
//
//  1. exit code == 2
//  2. stderr contains ≥ 1 JSON line where `level` == "ERROR" AND `msg` is
//     a non-empty string
//  3. the runtime-error keyword set {runtime, open, unable, no such}
//     appears in EITHER obj["msg"] OR obj["err"] — the slog convention is
//     `msg` = human-readable operation label, `err` = wrapped error
//     chain (runtimeError.Error() string lives in `err`, not `msg`)
//
// Interpretation note: parent-spec TC-CT-04 originally said "msg matches
// runtimeError.Error() format". That assumed slog would put the wrapped
// error string into the `msg` field. The actual production wiring puts
// the operation label in `msg` and the error chain in `err`. The keyword
// check applied to BOTH fields keeps the spec's INTENT (verify structured
// error logging) without forcing a literal field-name interpretation that
// the implementation never satisfied. See fix-spec `Notes` after merge.
//
// NOT parallel: mutates process env via t.Setenv. t.Setenv panics if
// t.Parallel was called, so this function MUST stay non-parallel.
func Test_TC_CT_04_runtime_error_exits_two(t *testing.T) {
	t.Setenv("LOG_FORMAT", "json")

	var stdout, stderr bytes.Buffer
	root := buildRoot(t)
	code := RunCmd(root, []string{
		"stats",
		"--db-path", "/nonexistent/dir/that/does/not/exist/db.sqlite",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2 for runtime DB error, got %d (stderr=%q)", code, stderr.String())
	}

	obj, ok := findJSONLogLine(stderr.Bytes(), "ERROR")
	if !ok {
		t.Fatalf("expected ≥1 JSON line on stderr with level=ERROR; full stderr:\n%s", stderr.String())
	}
	// slog convention: `msg` is a short human-readable operation label;
	// `err` carries the wrapped error chain. They have distinct roles, so we
	// assert each separately (per test-reviewer SHOULD FIX 2026-05-15):
	//
	//   (a) `msg` must be a non-empty string — verifies slog WAS invoked
	//       with an operation label (a fmt.Fprintf fallback would omit
	//       any `msg` field entirely).
	//   (b) `err` must contain at least one runtime/IO keyword from
	//       {runtime, open, unable, no such} — verifies the WRAPPED ERROR
	//       chain is captured, not just a generic label.
	//
	// The original parent-spec wording ("msg matches runtimeError.Error()
	// format `runtime: <msg>`") assumed slog records would put the wrapped
	// error string into `msg`. Production slog wiring puts the operation
	// label in `msg` and the chain in `err`. See spec
	// `.specs/fix-refactor-tools-learn-contract-test-rigor.md` Notes for
	// the interpretation note.
	msg, _ := obj["msg"].(string)
	if msg == "" {
		t.Fatalf("expected obj[\"msg\"] to be a non-empty string (slog operation label); got obj=%v; full stderr:\n%s", obj, stderr.String())
	}
	errField, _ := obj["err"].(string)
	if errField == "" {
		t.Fatalf("expected obj[\"err\"] to be a non-empty string (wrapped error chain); got obj=%v; full stderr:\n%s", obj, stderr.String())
	}
	if !strings.Contains(errField, "runtime") &&
		!strings.Contains(errField, "open") &&
		!strings.Contains(errField, "unable") &&
		!strings.Contains(errField, "no such") {
		t.Errorf("expected slog record's err field to contain one of "+
			"{runtime, open, unable, no such}; got msg=%q err=%q; full stderr:\n%s",
			msg, errField, stderr.String())
	}
}

// TC-CT-05: `learn` with no args exits 0 and prints usage to stdout.
func Test_TC_CT_05_no_args_prints_usage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	root := buildRoot(t)
	code := RunCmd(root, []string{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0 for no-args, got %d (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected stdout to contain \"Usage:\", got: %q", stdout.String())
	}
}

// TC-CT-06: init() registration intact — the set of registered subcommand
// names (excluding cobra-injected help/completion) equals the canonical
// 16-name set. SET equality, not count.
func Test_TC_CT_06_registration_set_equality(t *testing.T) {
	t.Parallel()
	root := buildRoot(t)

	want := make(map[string]bool, len(canonicalSubcommands))
	for _, n := range canonicalSubcommands {
		want[n] = true
	}

	got := make(map[string]bool)
	for _, c := range root.Commands() {
		name := c.Name()
		// Filter cobra-injected commands.
		if name == "help" || name == "completion" {
			continue
		}
		got[name] = true
	}

	for n := range want {
		if !got[n] {
			t.Errorf("missing registered subcommand %q", n)
		}
	}
	for n := range got {
		if !want[n] {
			t.Errorf("unexpected registered subcommand %q", n)
		}
	}
}

// TC-CT-07: post-TASK-7 layout invariant. `tools/learn/internal/` does not
// exist; root `.go` file count (excluding `_test.go`) is exactly 32 (31 from
// the original ledger + `doc.go` added during the refactor).
//
// Hard-asserts the count via t.Errorf — the spec REQ-1 ledger is precise, and
// silent drift (e.g. someone adding a fourth ingest file without updating the
// spec/ledger) is exactly the regression class this TC exists to catch.
// Adjusting the expected count requires a deliberate spec update.
func Test_TC_CT_07_layout_invariant(t *testing.T) {
	t.Parallel()
	pkgDir := locatePkgDir(t)

	internalDir := filepath.Join(pkgDir, "internal")
	if info, statErr := os.Stat(internalDir); statErr == nil && info.IsDir() {
		t.Errorf("expected %s to NOT exist after TASK-7; got dir", internalDir)
	}

	entries, readErr := os.ReadDir(pkgDir)
	if readErr != nil {
		t.Fatalf("read pkg dir %q: %v", pkgDir, readErr)
	}
	prodCount := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		prodCount++
	}
	const expected = 32
	if prodCount != expected {
		t.Errorf("production .go count at tools/learn/ root: got %d, want %d "+
			"(REQ-1 ledger drift — update the spec deliberately if intentional)",
			prodCount, expected)
	}
}

// TC-CT-08: every .go file at `tools/learn/` root declares `package learn` or
// `package learn_test`; `cmd/learn/main.go` declares `package main`.
func Test_TC_CT_08_package_declarations(t *testing.T) {
	t.Parallel()
	pkgDir := locatePkgDir(t)

	entries, readErr := os.ReadDir(pkgDir)
	if readErr != nil {
		t.Fatalf("read pkg dir: %v", readErr)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())
		decl, readDeclErr := readPackageDecl(path)
		if readDeclErr != nil {
			t.Errorf("read package decl %q: %v", path, readDeclErr)
			continue
		}
		if decl != "package learn" && decl != "package learn_test" {
			t.Errorf("%s: expected `package learn` or `package learn_test`, got %q", e.Name(), decl)
		}
	}

	mainPath := filepath.Join(pkgDir, "cmd", "learn", "main.go")
	if _, statErr := os.Stat(mainPath); statErr == nil {
		decl, readMainErr := readPackageDecl(mainPath)
		if readMainErr != nil {
			t.Errorf("read main.go package decl: %v", readMainErr)
		} else if decl != "package main" {
			t.Errorf("cmd/learn/main.go: expected `package main`, got %q", decl)
		}
	} else {
		t.Logf("cmd/learn/main.go not found at %s (skipping main-pkg check)", mainPath)
	}
}

// TC-CT-09: structured stderr output is valid JSON post-flatten. Trigger a
// subcommand that emits at least one slog record (stats against a broken db
// path), capture stderr, and require at least one line that parses as JSON.
// Sets LOG_FORMAT=json defensively even though the current handler is JSON by
// default — keeps the test honest if the env knob is ever added.
//
// NOT parallel: mutates process env via t.Setenv. t.Setenv forbids it.
//
// Uses the shared findJSONLogLine helper (also used by TC-CT-04) to honor the
// DRY motivation behind the helper extraction; level="" means "any level".
func Test_TC_CT_09_stderr_is_json(t *testing.T) {
	t.Setenv("LOG_FORMAT", "json")

	var stdout, stderr bytes.Buffer
	root := buildRoot(t)
	code := RunCmd(root, []string{
		"stats",
		"--db-path", "/nonexistent/dir/db.sqlite",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit triggering a log line, got 0 (stderr=%q)", stderr.String())
	}

	if _, ok := findJSONLogLine(stderr.Bytes(), ""); !ok {
		t.Errorf("expected at least one JSON line on stderr; got:\n%s", stderr.String())
	}
}

// findJSONLogLine scans the given stderr buffer line-by-line and returns
// the first line that is valid JSON AND (if level is non-empty) has
// obj["level"] == level. Lines that don't start with '{' are skipped.
// Callers should pass the full stderr to t.Errorf on miss for diagnostics.
//
// Shared between TC-CT-04 (asserts level=ERROR) and TC-CT-09 (any JSON line).
func findJSONLogLine(stderr []byte, level string) (map[string]any, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(stderr))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(line), &obj) != nil {
			continue
		}
		if level == "" {
			return obj, true
		}
		if lvl, _ := obj["level"].(string); lvl == level {
			return obj, true
		}
	}
	return nil, false
}

// TC-CT-10: unknown flag yields a usage error (exit 1).
func Test_TC_CT_10_unknown_flag(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	root := buildRoot(t)
	code := RunCmd(root, []string{"recall", "--unknown-flag", "x"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for unknown flag, got %d (stderr=%q)", code, stderr.String())
	}
	combined := stderr.String()
	if !strings.Contains(combined, "unknown flag") && !strings.Contains(combined, "unknown shorthand") {
		t.Errorf("expected stderr to contain \"unknown flag\" or \"unknown shorthand\", got: %q", combined)
	}
}

// TC-CT-11: `record-decision` without required flags surfaces a usage error
// from the subcommand's own validation, hitting *usageError → exit 1.
//
// The complementary cobra-native `MarkFlagRequired` path (REQ-2 of
// fix-cobra-error-exit-codes.md) is covered by Test_CobraRequiredFlag_exits_one
// in cli_test.go — kept separate because no learn subcommand currently uses
// MarkFlagRequired (they all validate in their own RunE, like record-decision
// does here).
func Test_TC_CT_11_record_decision_missing_required(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	root := buildRoot(t)
	code := RunCmd(root, []string{"record-decision"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for missing required flag, got %d (stderr=%q)", code, stderr.String())
	}
	combined := stderr.String()
	if strings.TrimSpace(combined) == "" {
		t.Fatalf("expected non-empty stderr for missing required flag")
	}
	if !strings.Contains(combined, "required") {
		t.Errorf("expected stderr to contain \"required\" (validation hint), got: %q", combined)
	}
}

// TC-CT-12: `--db-path` flag accepted by recall (no persistent root flag
// exists in the current implementation — this is the closest available
// surrogate for "persistent flag inheritance"). DEVIATION from task wording
// which uses `--db`; see report.
//
// The flag MUST parse cleanly. We accept exit 2 (runtime DB error, expected
// since the file does not exist) and exit 0 (defensively, if any future
// implementation auto-creates the file). Exit 1 (usage / flag parse failure)
// is the failure mode this TC guards against.
func Test_TC_CT_12_db_flag_parses(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test-x.db")
	var stdout, stderr bytes.Buffer
	root := buildRoot(t)
	code := RunCmd(root, []string{
		"recall",
		"--db-path", dbPath,
		"--prompt", "x",
	}, &stdout, &stderr)
	if code == 1 {
		t.Fatalf("expected flag to parse (exit != 1), got 1 (stderr=%q)", stderr.String())
	}
}

// locatePkgDir returns the absolute path of the `tools/learn` directory by
// walking up from the current working directory until it finds `go.mod`. We
// rely on `go test`'s convention of running with the package dir as CWD, but
// guard against test wrappers that nest CWD differently.
//
// Fails LOUD via t.Fatalf if the walk exhausts without finding go.mod — a
// silent CWD fallback would produce false-positive PASS in TC-CT-07 and
// TC-CT-08 under unusual test runners that set CWD outside the module tree.
func locatePkgDir(t *testing.T) string {
	t.Helper()
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatalf("locatePkgDir: os.Getwd: %v", wdErr)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("locatePkgDir: could not find go.mod ascending from %q after 8 levels", wd)
	return "" // unreachable; t.Fatalf does not return
}

// readPackageDecl reads the first `package <name>` line of a Go source file.
func readPackageDecl(path string) (string, error) {
	f, openErr := os.Open(path) //nolint:gosec // test reads files inside the package dir
	if openErr != nil {
		return "", openErr
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "package ") {
			// strip trailing comment, if any
			if idx := strings.Index(line, "//"); idx >= 0 {
				line = strings.TrimSpace(line[:idx])
			}
			return line, nil
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return "", scanErr
	}
	return "", nil
}
