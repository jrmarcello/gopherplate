package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// capabilityDoc holds the parsed state from a docs/capabilities/*.md file.
type capabilityDoc struct {
	name         string // filename without extension
	codePaths    []string
	lastVerified time.Time
	hasCodeSec   bool
}

// commitDateLookup is the injectable function for resolving a file's last git commit date.
// Returns (time.Time{}, false) on any failure → no staleness warn.
type commitDateLookup func(absPath string) (time.Time, bool)

// defaultCommitDateLookup runs `git log -1 --format=%cI -- <absPath>` and parses the result.
func defaultCommitDateLookup(absPath string) (time.Time, bool) {
	cmd := exec.Command("git", "log", "-1", "--format=%cI", "--", absPath) //nolint:gosec // G204: path is resolved from a capability doc code listing, not raw user input
	cmd.Stderr = io.Discard
	out, gitErr := cmd.Output()
	if gitErr != nil {
		return time.Time{}, false
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return time.Time{}, false
	}
	t, parseErr := time.Parse(time.RFC3339, s)
	if parseErr != nil {
		return time.Time{}, false
	}
	return t, true
}

// isStale returns true iff lastCommit is strictly NEWER than lastVerified.
// Equal or older → false (no warn).
func isStale(lastVerified, lastCommit time.Time) bool {
	return lastCommit.After(lastVerified)
}

// parseCapabilityDoc parses a docs/capabilities/*.md file for the capabilities subcommand.
// It extracts:
//   - ## Code section: a bullet list of paths + a `Last-verified: <date> (<commit>)` line
//   - hasCodeSec: true iff a ## Code heading was found
func parseCapabilityDoc(name, content string) capabilityDoc {
	doc := capabilityDoc{name: name}
	lines := strings.Split(content, "\n")
	inCode := false
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "## Code") {
			inCode = true
			doc.hasCodeSec = true
			continue
		}
		if inCode {
			if strings.HasPrefix(trimmed, "## ") {
				inCode = false
				continue
			}
			if strings.HasPrefix(trimmed, "- ") {
				rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				// Bullet format is "<path> — <human description>". A path never
				// contains spaces, so it is the first whitespace-delimited field;
				// strip surrounding backticks (the TEMPLATE wraps placeholders in `…`).
				if fields := strings.Fields(rest); len(fields) > 0 {
					path := strings.Trim(fields[0], "`")
					if path != "" {
						doc.codePaths = append(doc.codePaths, path)
					}
				}
				continue
			}
			if strings.HasPrefix(trimmed, "Last-verified:") {
				rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "Last-verified:"))
				// Format: "2026-01-01 (abc1234)"
				if idx := strings.Index(rest, " ("); idx > 0 {
					dateStr := strings.TrimSpace(rest[:idx])
					t, parseErr := time.Parse("2006-01-02", dateStr)
					if parseErr == nil {
						doc.lastVerified = t
					}
				}
			}
		}
	}
	return doc
}

// runCapabilities implements the `capabilities [dir]` subcommand.
func runCapabilities(dir string, lookup commitDateLookup) int {
	if lookup == nil {
		lookup = defaultCommitDateLookup
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		fmt.Fprintf(os.Stderr, "error: reading capabilities dir %q: %v\n", dir, readErr)
		return 2
	}

	hasError := false

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		// Skip README.md, TEMPLATE.md, MANIFEST.md
		if name == "README.md" || name == "TEMPLATE.md" || name == "MANIFEST.md" {
			continue
		}
		// Skip symlinks
		info, statErr := e.Info()
		if statErr != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		docPath := filepath.Join(dir, name)
		docName := strings.TrimSuffix(name, ".md")
		data, readFileErr := os.ReadFile(docPath) //nolint:gosec // G304: path is constructed from ReadDir result, not raw user input
		if readFileErr != nil {
			fmt.Fprintf(os.Stderr, "%s: error: reading file: %v\n", docPath, readFileErr)
			hasError = true
			continue
		}

		doc := parseCapabilityDoc(docName, string(data))

		if !doc.hasCodeSec {
			fmt.Printf("%s: warning: no ## Code section found\n", docPath)
			continue
		}

		for _, codePath := range doc.codePaths {
			absPath, absErr := filepath.Abs(codePath)
			if absErr != nil {
				absPath = codePath
			}
			if _, statErr2 := os.Stat(absPath); os.IsNotExist(statErr2) {
				fmt.Printf("%s: error: capability %s: code path %q does not exist\n", docPath, docName, codePath)
				hasError = true
				continue
			}
			// Staleness check
			if !doc.lastVerified.IsZero() {
				commitTime, ok := lookup(absPath)
				if ok && isStale(doc.lastVerified, commitTime) {
					fmt.Printf("%s: warning: capability %s: code path %q has commits newer than Last-verified %s\n",
						docPath, docName, codePath, doc.lastVerified.Format("2006-01-02"))
				}
			}
		}
	}

	if hasError {
		return 1
	}
	return 0
}
