package main

import (
	"fmt"
	"os"
	"sort"
)

// collectFiles returns a sorted, deduplicated union of all file paths across tasks.
// Tasks with execOnly=true contribute no paths.
func collectFiles(m model) []string {
	seen := make(map[string]bool)
	for _, t := range m.tasks {
		if t.execOnly {
			continue
		}
		for _, f := range t.files {
			seen[f] = true
		}
	}
	result := make([]string, 0, len(seen))
	for f := range seen {
		result = append(result, f)
	}
	sort.Strings(result)
	return result
}

// runFiles implements the `files <spec...>` subcommand.
func runFiles(paths []string) int {
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "error: files: no spec files specified")
		return 2
	}
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
		for _, f := range collectFiles(m) {
			fmt.Println(f)
		}
	}
	return 0
}
