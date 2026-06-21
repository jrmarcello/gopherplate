package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// capability represents a parsed capability document.
type capability struct {
	name           string // filename without extension
	slug           string
	status         string
	source         string
	guaranteedReqs []string
}

// parseCapability parses a capability markdown file's content.
// On parse failure it returns a capability with <missing> placeholders.
func parseCapability(name, content string) (capability, bool) {
	capDoc := capability{name: name}
	lines := strings.Split(content, "\n")

	inReqs := false // only collect guaranteed-REQ items inside ## Guaranteed Requirements
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		switch {
		case strings.HasPrefix(trimmed, "## Slug: "):
			capDoc.slug = strings.TrimSpace(strings.TrimPrefix(trimmed, "## Slug: "))
			inReqs = false
		case strings.HasPrefix(trimmed, "## Status: "):
			capDoc.status = strings.TrimSpace(strings.TrimPrefix(trimmed, "## Status: "))
			inReqs = false
		case strings.HasPrefix(trimmed, "## Guaranteed Requirements"):
			inReqs = true
		case strings.HasPrefix(trimmed, "## "):
			inReqs = false
		case inReqs && strings.HasPrefix(trimmed, "- "):
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if strings.Contains(val, "-REQ-") || strings.HasPrefix(val, "REQ-") {
				capDoc.guaranteedReqs = append(capDoc.guaranteedReqs, val)
			}
		}
		// Source is captured by the dedicated inSource scan below.
	}

	// Parse source: find Source section and grab text after it
	inSource := false
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "## Source") {
			inSource = true
			continue
		}
		if inSource {
			if strings.HasPrefix(trimmed, "## ") {
				inSource = false
				continue
			}
			if v := strings.TrimSpace(line); v != "" {
				capDoc.source = v
				inSource = false
			}
		}
	}

	malformed := capDoc.slug == "" || capDoc.status == ""
	return capDoc, !malformed
}

// renderManifest generates a deterministic markdown manifest table.
func renderManifest(caps []capability) string {
	// Sort by slug for determinism; fall back to name for missing slug
	sorted := make([]capability, len(caps))
	copy(sorted, caps)
	sort.Slice(sorted, func(i, j int) bool {
		si := sorted[i].slug
		if si == "" {
			si = sorted[i].name
		}
		sj := sorted[j].slug
		if sj == "" {
			sj = sorted[j].name
		}
		return si < sj
	})

	var sb strings.Builder
	sb.WriteString("# Capabilities Manifest\n\n")
	sb.WriteString("| Name | Slug | Status | Guaranteed REQs |\n")
	sb.WriteString("| --- | --- | --- | --- |\n")
	for _, c := range sorted {
		slug := c.slug
		if slug == "" {
			slug = "<missing>"
		}
		status := c.status
		if status == "" {
			status = "<missing>"
		}
		reqs := strings.Join(c.guaranteedReqs, ", ")
		if reqs == "" {
			reqs = "-"
		}
		fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n", c.name, slug, status, reqs)
	}
	return sb.String()
}

// runManifest implements the manifest subcommand.
// capsDir is the directory to scan (docs/capabilities/).
// write: if non-empty, write output to that path instead of stdout.
func runManifest(capsDir, writePath string) error {
	entries, readErr := os.ReadDir(capsDir)
	if readErr != nil {
		return fmt.Errorf("reading capabilities dir %q: %w", capsDir, readErr)
	}

	var caps []capability
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		// Skip the generated manifest and the non-capability docs (index + template),
		// so only real capability docs are listed.
		if name == "MANIFEST.md" || name == "README.md" || name == "TEMPLATE.md" {
			continue
		}
		baseName := strings.TrimSuffix(name, ".md")
		path := filepath.Join(capsDir, name)
		data, readFileErr := os.ReadFile(path)
		if readFileErr != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, readFileErr)
			continue
		}
		capDoc, ok := parseCapability(baseName, string(data))
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: capability doc %s is malformed (missing Slug or Status), using placeholders\n", path)
		}
		caps = append(caps, capDoc)
	}

	output := renderManifest(caps)

	if writePath != "" {
		writeErr := os.WriteFile(writePath, []byte(output), 0o644)
		if writeErr != nil {
			return fmt.Errorf("writing manifest to %q: %w", writePath, writeErr)
		}
		return nil
	}

	fmt.Print(output)
	return nil
}
