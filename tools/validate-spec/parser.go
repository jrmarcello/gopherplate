package main

import (
	"regexp"
	"strings"
)

// requirement is a single REQ declaration parsed from the Requirements section.
type requirement struct {
	id            string
	line          int
	noTestPresent bool
	noTestReason  string
}

// testCase is a single TC declaration parsed from the Test Plan table.
type testCase struct {
	id     string
	reqRef string
	line   int
}

// task is a single task declaration from the Tasks section.
type task struct {
	id       string
	files    []string
	tests    []string
	depends  []string
	line     int
	execOnly bool
}

// batch is a single parallel batch declaration.
type batch struct {
	index          int
	taskIDs        []string
	sharedAdditive []string
	line           int
}

// model is the parsed representation of a spec document.
type model struct {
	status          string
	slug            string
	statusLineCount int
	slugLineCount   int
	statusRaw       string // raw value after "Status: " on first occurrence
	sections        map[string]bool
	reqs            []requirement
	tcs             []testCase
	tasks           []task
	batches         []batch
	bodyStripped    string // content with HTML comments removed — for the capability-link check
	statusLine      int    // 1-based line of the first ## Status: heading
	slugLine        int    // 1-based line of the first ## Slug: heading
}

var (
	reReqDecl   = regexp.MustCompile(`^- \[[ x]\] ([A-Za-z0-9][A-Za-z0-9_-]*): `)
	reNoTest    = regexp.MustCompile(`\(no-test:([^)]*)\)`)
	reTCRow     = regexp.MustCompile(`^\| ([^\|]+) \| ([^\|]+) \|`)
	reTaskDecl  = regexp.MustCompile(`^- \[[ x]\] (TASK-(?:\d+|[A-Z][A-Z0-9]*(?:-[A-Z][A-Z0-9]*)*)): `)
	reBatchLine = regexp.MustCompile(`^Batch (\d+): \[([^\]]*)\]`)
	reSharedAdd = regexp.MustCompile(`shared-additive:\s*(.+)`)
	reReqID     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*-REQ-\d+$|^REQ-\d+$`)
	reTCID      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*-TC-[A-Z0-9]+-\d+$|^TC-[A-Z0-9]+-\d+$`)
)

// parseSpec parses the content of a spec markdown file into a model.
// It never returns a non-nil error for structural issues — those become findings; the error is
// reserved for future unrecoverable input and is currently always nil (callers pre-decode the file).
//
//nolint:unparam // error return reserved per the doc above; currently always nil.
func parseSpec(content string) (model, error) {
	m := model{
		sections: make(map[string]bool),
	}

	lines := strings.Split(content, "\n")

	// Fence state
	inFence := false
	fenceMarker := "" // "```" or "~~~"
	// HTML comment state (<!-- ... -->). Spec templates use comments heavily, and they
	// routinely contain `## ` headings and REQ/TC examples that must NOT be parsed as structure.
	inComment := false
	var bodyB strings.Builder // accumulates comment-stripped content for the capability-link check

	const (
		secNone            = ""
		secRequirements    = "Requirements"
		secTestPlan        = "Test Plan"
		secTasks           = "Tasks"
		secParallelBatches = "Parallel Batches"
	)
	currentSection := secNone
	var currentTask *task

	for lineNum, rawLine := range lines {
		lineNo := lineNum + 1
		trimmed := strings.TrimLeft(rawLine, " \t")
		leadingSpaces := len(rawLine) - len(trimmed)

		// ---- Fence detection (up to 3 leading spaces) ----
		if leadingSpaces <= 3 {
			if !inFence {
				if strings.HasPrefix(trimmed, "```") {
					inFence = true
					fenceMarker = "```"
					continue
				}
				if strings.HasPrefix(trimmed, "~~~") {
					inFence = true
					fenceMarker = "~~~"
					continue
				}
			} else {
				// Closing fence must use same marker
				if fenceMarker == "```" && strings.HasPrefix(trimmed, "```") {
					inFence = false
					fenceMarker = ""
					continue
				}
				if fenceMarker == "~~~" && strings.HasPrefix(trimmed, "~~~") {
					inFence = false
					fenceMarker = ""
					continue
				}
			}
		}

		// ---- HTML comment stripping (outside fences only; fenced content is opaque) ----
		// Remove <!-- ... --> spans so headings / REQ-TC examples inside comments are never
		// parsed as structure. Multi-line comments carry state via inComment. Content before/
		// after a comment on the same line is preserved.
		if !inFence {
			rawLine, inComment = stripComments(rawLine, inComment)
			trimmed = strings.TrimLeft(rawLine, " \t")
		}
		bodyB.WriteString(rawLine)
		bodyB.WriteByte('\n')

		// ---- Batch lines: parse regardless of fence state, but never inside a comment ----
		if bm := reBatchLine.FindStringSubmatch(trimmed); bm != nil {
			b := batch{
				index:   parseInt(bm[1]),
				taskIDs: parseBracketList(bm[2]),
				line:    lineNo,
			}
			m.batches = append(m.batches, b)
			// Don't continue — fall through (if in fence, skip further; else handle section)
		}

		// ---- While inside a fence or an open HTML comment, skip structural matching ----
		if inFence || inComment {
			continue
		}

		// ---- Section heading ----
		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.TrimPrefix(trimmed, "## ")

			if strings.HasPrefix(heading, "Status: ") {
				m.statusLineCount++
				if m.statusLineCount == 1 {
					m.statusLine = lineNo
					m.statusRaw = strings.TrimPrefix(heading, "Status: ")
					fields := strings.Fields(m.statusRaw)
					if len(fields) > 0 {
						m.status = fields[0]
					}
				}
				finalizeTask(&m, &currentTask)
				continue
			}

			if strings.HasPrefix(heading, "Slug: ") {
				m.slugLineCount++
				if m.slugLineCount == 1 {
					m.slugLine = lineNo
					m.slug = strings.TrimSpace(strings.TrimPrefix(heading, "Slug: "))
				}
				finalizeTask(&m, &currentTask)
				continue
			}

			// Generic section
			finalizeTask(&m, &currentTask)
			sectionName := heading
			m.sections[sectionName] = true
			currentSection = sectionName
			continue
		}

		if strings.TrimSpace(rawLine) == "" {
			continue
		}

		switch currentSection {
		case secRequirements:
			parseReqLine(trimmed, lineNo, &m)

		case secTestPlan:
			parseTCLine(trimmed, lineNo, &m)

		case secTasks:
			if tm := reTaskDecl.FindStringSubmatch(trimmed); tm != nil {
				finalizeTask(&m, &currentTask)
				currentTask = &task{id: tm[1], line: lineNo}
			} else if currentTask != nil {
				parseTaskSubItem(trimmed, currentTask)
			}

		case secParallelBatches:
			if sm := reSharedAdd.FindStringSubmatch(trimmed); sm != nil {
				if len(m.batches) > 0 {
					files := parseFileList(sm[1])
					last := len(m.batches) - 1
					m.batches[last].sharedAdditive = append(m.batches[last].sharedAdditive, files...)
				}
			}
		}
	}

	finalizeTask(&m, &currentTask)
	m.bodyStripped = bodyB.String()
	return m, nil
}

func finalizeTask(m *model, t **task) {
	if *t != nil {
		m.tasks = append(m.tasks, **t)
		*t = nil
	}
}

// stripComments removes HTML comment regions (<!-- ... -->) from a line, carrying
// multi-line comment state across calls. Returns the line with commented spans removed
// and the resulting comment state. Content before/after a comment on the same line is kept,
// so an inline `## Status: Active <!-- note -->` keeps `## Status: Active`.
func stripComments(line string, inComment bool) (string, bool) {
	var b strings.Builder
	work := line
	for {
		if inComment {
			idx := strings.Index(work, "-->")
			if idx < 0 {
				return b.String(), true
			}
			work = work[idx+len("-->"):]
			inComment = false
			continue
		}
		idx := strings.Index(work, "<!--")
		if idx < 0 {
			b.WriteString(work)
			return b.String(), false
		}
		b.WriteString(work[:idx])
		work = work[idx+len("<!--"):]
		inComment = true
	}
}

func parseReqLine(line string, lineNo int, m *model) {
	match := reReqDecl.FindStringSubmatch(line)
	if match == nil {
		return
	}
	reqID := match[1]
	// Validate it looks like a REQ ID (contains -REQ- or is REQ-\d+)
	if !reReqID.MatchString(reqID) {
		return
	}
	r := requirement{id: reqID, line: lineNo}
	if ntm := reNoTest.FindStringSubmatch(line); ntm != nil {
		r.noTestPresent = true
		r.noTestReason = strings.TrimSpace(ntm[1])
	}
	m.reqs = append(m.reqs, r)
}

func parseTCLine(line string, lineNo int, m *model) {
	// Must start with | and have at least two columns
	match := reTCRow.FindStringSubmatch(line)
	if match == nil {
		return
	}
	tcID := strings.TrimSpace(match[1])
	reqRef := strings.TrimSpace(match[2])

	// Skip header and separator rows
	if tcID == "TC-ID" || tcID == "TC" || strings.HasPrefix(tcID, "---") || strings.HasPrefix(tcID, "===") {
		return
	}

	// Only accept IDs that look like TC IDs
	if !reTCID.MatchString(tcID) {
		return
	}

	m.tcs = append(m.tcs, testCase{id: tcID, reqRef: reqRef, line: lineNo})
}

func parseTaskSubItem(line string, t *task) {
	trimmed := strings.TrimSpace(line)
	if after, ok := strings.CutPrefix(trimmed, "- files:"); ok {
		val := strings.TrimSpace(after)
		if val == "(none — execution only)" || val == "(none)" {
			t.execOnly = true
			return
		}
		if val != "" {
			t.files = append(t.files, parseFileList(val)...)
		}
	} else if after, ok := strings.CutPrefix(trimmed, "- tests:"); ok {
		val := strings.TrimSpace(after)
		t.tests = append(t.tests, parseCommaList(val)...)
	} else if after, ok := strings.CutPrefix(trimmed, "- depends:"); ok {
		val := strings.TrimSpace(after)
		t.depends = append(t.depends, parseCommaList(val)...)
	}
}

func parseBracketList(s string) []string {
	return parseCommaList(s)
}

func parseCommaList(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parseFileList(s string) []string {
	if strings.Contains(s, ",") {
		return parseCommaList(s)
	}
	var result []string
	for _, p := range strings.Fields(s) {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
