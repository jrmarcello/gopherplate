package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// reSentinel matches lines like: var ErrFoo = errors.New(  (captures "ErrFoo")
var reSentinel = regexp.MustCompile(`^\s*(?:var\s+)?(Err[A-Za-z0-9]+)\s*=\s*errors\.New\(`)

// renderBootstrap renders a skeleton capability doc for the given package.
// This is a pure function (no I/O): injectable for tests.
func renderBootstrap(name string, files, errs []string, date, commit string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Capability: %s\n\n", name)
	sb.WriteString("## Slug: TODO\n\n")
	sb.WriteString("## Status: Active\n\n")
	sb.WriteString("## Source\n\n")
	sb.WriteString("- Specs: TODO\n")
	sb.WriteString("- ADRs: TODO\n\n")
	sb.WriteString("## Code\n\n")
	for _, f := range files {
		fmt.Fprintf(&sb, "- %s\n", f)
	}
	fmt.Fprintf(&sb, "\nLast-verified: %s (%s)\n\n", date, commit)
	sb.WriteString("## Guarantees (current truth)\n\nTODO\n\n")
	sb.WriteString("## Invariants\n\nTODO\n\n")
	sb.WriteString("## Error Contract\n\n")
	for _, e := range errs {
		fmt.Fprintf(&sb, "- %s\n", e)
	}
	sb.WriteString("\n## Changelog\n")
	return sb.String()
}

// runBootstrap implements the `bootstrap-capability <pkg>` subcommand.
func runBootstrap(pkgDir string) int {
	entries, readErr := os.ReadDir(pkgDir)
	if readErr != nil {
		fmt.Fprintf(os.Stderr, "error: reading package dir %q: %v\n", pkgDir, readErr)
		return 2
	}

	var goFiles []string
	var errNames []string

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// Skip symlinks
		if e.Type()&fs.ModeSymlink != 0 {
			continue
		}

		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		// Skip test files
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		goFiles = append(goFiles, name)

		// Grep for sentinel errors
		path := filepath.Join(pkgDir, name)
		sentinels, grepErr := grepSentinels(path)
		if grepErr == nil {
			errNames = append(errNames, sentinels...)
		}
	}

	sort.Strings(goFiles)
	sort.Strings(errNames)

	// Deduplicate
	errNames = dedup(errNames)

	pkgName := filepath.Base(pkgDir)
	output := renderBootstrap(pkgName, goFiles, errNames, "TODO", "TODO")
	fmt.Print(output)
	return 0
}

func grepSentinels(path string) ([]string, error) {
	f, openErr := os.Open(path) //nolint:gosec // G304: path is constructed from ReadDir result within a user-specified package dir
	if openErr != nil {
		return nil, openErr
	}
	defer func() { _ = f.Close() }()

	var names []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := reSentinel.FindStringSubmatch(line); m != nil {
			names = append(names, m[1])
		}
	}
	return names, scanner.Err()
}

func dedup(s []string) []string {
	seen := make(map[string]bool)
	result := s[:0]
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
