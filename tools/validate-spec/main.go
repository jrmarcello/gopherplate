package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(run(os.Args))
}

// run is the testable entry point. Returns exit code.
func run(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: validate-spec <spec.md> [spec.md ...] | manifest [caps-dir] [--write] | files <spec.md> [spec.md ...] | capabilities [dir] | bootstrap-capability <pkg>")
		return 2
	}

	subcommand := args[1]

	switch subcommand {
	case "manifest":
		return runManifestCmd(args[2:])
	case "files":
		return runFiles(args[2:])
	case "capabilities":
		dir := "docs/capabilities"
		if len(args) > 2 {
			dir = args[2]
		}
		return runCapabilities(dir, nil)
	case "bootstrap-capability":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "error: bootstrap-capability: missing <pkg> argument")
			return 2
		}
		return runBootstrap(args[2])
	case "--help", "-h", "help":
		fmt.Fprintln(os.Stderr, "usage: validate-spec <spec.md> [spec.md ...] | manifest [caps-dir] [--write] | files <spec.md> [spec.md ...] | capabilities [dir] | bootstrap-capability <pkg>")
		return 0
	default:
		// Treat all args[1:] as spec file paths
		return runLint(args[1:])
	}
}

// runManifestCmd parses `manifest [caps-dir] [--write]`.
// caps-dir defaults to docs/capabilities; --write writes <caps-dir>/MANIFEST.md
// (otherwise the rendered manifest is printed to stdout). This matches REQ-7:
// `validate-spec manifest --write` scans docs/capabilities/*.md → docs/capabilities/MANIFEST.md.
func runManifestCmd(args []string) int {
	capsDir := "docs/capabilities"
	write := false
	dirSet := false

	for _, a := range args {
		switch {
		case a == "--write":
			write = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", a)
			return 2
		default:
			if dirSet {
				fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", a)
				return 2
			}
			capsDir = a
			dirSet = true
		}
	}

	writePath := ""
	if write {
		writePath = filepath.Join(capsDir, "MANIFEST.md")
	}

	if manifestErr := runManifest(capsDir, writePath); manifestErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", manifestErr)
		return 2
	}
	return 0
}

func runLint(paths []string) int {
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "error: no spec files specified")
		return 2
	}

	errorCount := 0
	warnCount := 0
	hasError := false

	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error: reading spec %q: %v\n", path, readErr)
			return 2
		}

		m, parseErr := parseSpec(string(data))
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "error: parsing spec %q: %v\n", path, parseErr)
			return 2
		}

		findings := validate(m)
		for _, f := range findings {
			fmt.Printf("%s:%d: %s: %s\n", path, f.line, f.sev, f.msg)
			switch f.sev {
			case severityError:
				errorCount++
				hasError = true
			case severityWarn:
				warnCount++
			}
		}
	}

	fmt.Fprintf(os.Stderr, "%d error(s), %d warning(s)\n", errorCount, warnCount)

	if hasError {
		return 1
	}
	return 0
}
