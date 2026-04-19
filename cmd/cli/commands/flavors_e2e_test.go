package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// cliBinaryOnce builds the gopherplate CLI once per test run and caches the
// binary path. Much faster than `go run` for every subprocess E2E, and avoids
// the "go.mod not found" pitfall when running from arbitrary cwd.
var (
	cliBinaryOnce sync.Once
	cliBinaryPath string
	cliBinaryErr  error
)

func buildCLIBinary(t *testing.T) string {
	t.Helper()
	cliBinaryOnce.Do(func() {
		repoRoot := findRepoRoot(t)
		outDir, mkErr := os.MkdirTemp("", "gopherplate-cli-*")
		if mkErr != nil {
			cliBinaryErr = fmt.Errorf("mktemp: %w", mkErr)
			return
		}
		bin := filepath.Join(outDir, "gopherplate")
		build := exec.Command("go", "build", "-o", bin, "./cmd/cli")
		build.Dir = repoRoot
		if out, runErr := build.CombinedOutput(); runErr != nil {
			cliBinaryErr = fmt.Errorf("build CLI: %w\n%s", runErr, out)
			return
		}
		cliBinaryPath = bin
	})
	if cliBinaryErr != nil {
		t.Fatalf("%v", cliBinaryErr)
	}
	return cliBinaryPath
}

// TestE2E_NewFlavorCrud_Builds scaffolds a full service with --flavor crud and
// verifies the output compiles (TC-E2E-01 + TC-E2E-04 partial). `make lint` is
// not run in-test because golangci-lint is installed only in CI; the CI job
// for this repo runs it separately on the scaffold via the perf-regression
// workflow.
//
// This is a real E2E: it runs `go run ./cmd/cli new ...` as a subprocess,
// pointing at the repo root as the template. Slow (~20s) — gated behind
// -short.
func TestE2E_NewFlavorCrud_Builds(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E scaffold test — run without -short to exercise")
	}

	bin := buildCLIBinary(t)
	repoRoot := findRepoRoot(t)
	work := t.TempDir()
	serviceName := "e2e-crud-demo"

	scaffoldCmd := exec.Command( //nolint:gosec // inputs are test-literal constants
		bin, "new", serviceName,
		"--flavor", "crud",
		"--module", "github.com/e2e/"+serviceName,
		"--template", repoRoot,
		"--yes",
		"--no-auth",
		"--no-idempotency",
		"--no-redis",
	)
	scaffoldCmd.Dir = work
	scaffoldCmd.Env = append(os.Environ(), "HOME="+work, "GOFLAGS=") // isolate from user env
	out, runErr := scaffoldCmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("scaffold failed: %v\n%s", runErr, out)
	}

	generated := filepath.Join(work, serviceName)
	if _, statErr := os.Stat(filepath.Join(generated, "go.mod")); statErr != nil {
		t.Fatalf("go.mod missing in scaffold: %v", statErr)
	}

	// TC-UC-07: k6 baseline is part of the CRUD scaffold (inherited from base).
	if _, statErr := os.Stat(filepath.Join(generated, "tests/load/baselines/load.json")); statErr != nil {
		t.Errorf("expected k6 baseline at tests/load/baselines/load.json (part of base scaffold, inherited by crud): %v", statErr)
	}

	// The post-scaffold `go build ./...` already ran and was reported in the
	// CLI output — assert the summary line so regressions in that path are
	// obvious.
	if !strings.Contains(string(out), "Running go build") {
		t.Errorf("scaffold output did not run smoke build: %s", out)
	}
}

// TestE2E_NewFlavorUnknown_Fails is TC-E2E-07 as a subprocess E2E.
func TestE2E_NewFlavorUnknown_Fails(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E subprocess test — run without -short")
	}
	bin := buildCLIBinary(t)
	repoRoot := findRepoRoot(t)
	work := t.TempDir()

	cmd := exec.Command( //nolint:gosec // test-literal constants
		bin, "new", "demo",
		"--flavor", "nonexistent",
		"--module", "github.com/e2e/demo",
		"--template", repoRoot,
		"--yes",
	)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "HOME="+work)
	out, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected non-zero exit for --flavor nonexistent, output: %s", out)
	}
	if !strings.Contains(string(out), "nonexistent") {
		t.Errorf("error output should mention invalid flavor, got: %s", out)
	}
	if !strings.Contains(string(out), "crud") {
		t.Errorf("error output should list crud as available, got: %s", out)
	}
}

// TestE2E_NewHelp_ShowsFlavor is TC-E2E-08 as a subprocess E2E.
func TestE2E_NewHelp_ShowsFlavor(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E subprocess test — run without -short")
	}
	bin := buildCLIBinary(t)
	repoRoot := findRepoRoot(t)
	work := t.TempDir()

	cmd := exec.Command(bin, "new", "--help") //nolint:gosec // test-literal
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "HOME="+work)
	out, _ := cmd.CombinedOutput() // --help exits 0 or 2 depending on cobra version
	help := string(out)

	if !strings.Contains(help, "--flavor") {
		t.Errorf("--help should document --flavor, got:\n%s", help)
	}
	if !strings.Contains(help, "crud") {
		t.Errorf("--help should list crud flavor, got:\n%s", help)
	}
}

// TestE2E_NewInvalidServiceName_Fails is TC-E2E-12 as a subprocess E2E.
func TestE2E_NewInvalidServiceName_Fails(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E subprocess test — run without -short")
	}
	bin := buildCLIBinary(t)
	repoRoot := findRepoRoot(t)
	work := t.TempDir()

	cmd := exec.Command( //nolint:gosec // test-literal
		bin, "new", "Invalid-Name-With-Uppercase",
		"--flavor", "crud",
		"--template", repoRoot,
		"--yes",
	)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "HOME="+work)
	out, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected non-zero exit for invalid service name, output: %s", out)
	}
	if !strings.Contains(string(out), "invalid service name") {
		t.Errorf("error should say 'invalid service name', got: %s", out)
	}
}

// TestE2E_NewTargetExists_Fails is TC-E2E-11 as a subprocess E2E.
func TestE2E_NewTargetExists_Fails(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E subprocess test — run without -short")
	}
	bin := buildCLIBinary(t)
	repoRoot := findRepoRoot(t)
	work := t.TempDir()
	serviceName := "e2e-exists"

	// Pre-create the target directory to simulate a collision.
	preExisting := filepath.Join(work, serviceName)
	if mkErr := os.MkdirAll(preExisting, 0o750); mkErr != nil {
		t.Fatalf("pre-create: %v", mkErr)
	}

	cmd := exec.Command( //nolint:gosec // test-literal
		bin, "new", serviceName,
		"--flavor", "crud",
		"--template", repoRoot,
		"--yes",
	)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "HOME="+work)
	out, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected non-zero exit when target dir exists, output: %s", out)
	}
	if !strings.Contains(string(out), "already exists") {
		t.Errorf("error should mention 'already exists', got: %s", out)
	}
}

// findRepoRoot walks up from the current working directory until it finds a
// go.mod whose module is gopherplate's — the same heuristic the CLI itself uses.
// Lets these tests run from any package-level invocation.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, cwdErr := os.Getwd()
	if cwdErr != nil {
		t.Fatalf("getwd: %v", cwdErr)
	}
	for {
		data, readErr := os.ReadFile(filepath.Join(dir, "go.mod")) //nolint:gosec // local test fixture
		if readErr == nil && strings.Contains(string(data), "module github.com/jrmarcello/gopherplate") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate gopherplate repo root from %s", dir)
		}
		dir = parent
	}
}

// Ensure fmt is exercised when t.Skip above masks unreachable branches.
var _ = fmt.Sprintf
