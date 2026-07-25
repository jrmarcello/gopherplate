package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type severity int

const (
	severityWarn severity = iota
	severityError
)

func (s severity) String() string {
	switch s {
	case severityWarn:
		return "warning"
	case severityError:
		return "error"
	default:
		return "unknown"
	}
}

type finding struct {
	line int
	sev  severity
	msg  string
}

// requiredSections that every spec must have.
var requiredSectionNames = []string{
	"Status",
	"Context",
	"Requirements",
	"Test Plan",
	"Design",
	"Tasks",
	"Parallel Batches",
	"Validation Criteria",
	"Execution Log",
}

// validStatuses is the set of allowed status token values.
var validStatuses = map[string]bool{
	"DRAFT":       true,
	"APPROVED":    true,
	"IN_PROGRESS": true,
	"DONE":        true,
	"FAILED":      true,
	"SUPERSEDED":  true,
	"ARCHIVED":    true,
}

// registeredTCTypes is the set of allowed TC type segments.
var registeredTCTypes = map[string]bool{
	"D":   true,
	"UC":  true,
	"E2E": true,
	"S":   true,
	"SH":  true,
	"CT":  true,
}

// reSlugFormat validates a declared slug value.
var reSlugFormat = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)

// reCapLink matches a docs/capabilities/*.md reference in the (comment-stripped) spec body.
var reCapLink = regexp.MustCompile(`docs/capabilities/[^/\s]+\.md`)

// validate runs all validators and returns findings sorted by (line, msg).
func validate(m model) []finding {
	type validatorFn func(model) []finding

	validators := []validatorFn{
		validateRequiredSections,
		validateStatusHeader,
		validateSlugDeclared,
		validateReqCovered,
		validateTaskHasFiles,
		validateGoProdTaskHasTests,
		validateDependsDAG,
		validateDependsExist,
		validateBatchMembership,
		validateBatchFileOverlap,
		validateBatchOrder,
		validateIDFormat,
		validateIDUnique,
		validateTCRefValid,
		validateTestsRefValid,
		validateTCReferenced,
		validateCapabilityLinked,
	}

	all := make([]finding, 0, len(validators))
	for _, fn := range validators {
		all = append(all, fn(m)...)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].line != all[j].line {
			return all[i].line < all[j].line
		}
		return all[i].msg < all[j].msg
	})
	return all
}

func validateRequiredSections(m model) []finding {
	var fs []finding
	// "Status" section presence is tracked via statusLineCount; we need the heading too.
	// Section headings other than Status/Slug are stored in m.sections.
	// We check: Status (via m.statusLineCount > 0), and others via sections.
	if m.statusLineCount == 0 {
		fs = append(fs, finding{line: 1, sev: severityError, msg: "missing required section: Status"})
	}
	for _, name := range requiredSectionNames {
		if name == "Status" {
			continue // handled above
		}
		if !m.sections[name] {
			fs = append(fs, finding{line: 1, sev: severityError, msg: fmt.Sprintf("missing required section: %s", name)})
		}
	}
	return fs
}

func validateStatusHeader(m model) []finding {
	var fs []finding
	ln := m.statusLine
	if ln == 0 {
		ln = 1
	}

	if m.statusLineCount == 0 {
		fs = append(fs, finding{line: 1, sev: severityError, msg: "no ## Status: line found"})
		return fs
	}

	if m.statusLineCount > 1 {
		fs = append(fs, finding{line: ln, sev: severityError, msg: fmt.Sprintf("duplicate ## Status: lines (%d found, expected 1)", m.statusLineCount)})
		return fs
	}

	// Exactly one status line. Check the state token.
	stateToken := m.status
	if !validStatuses[stateToken] {
		fs = append(fs, finding{line: ln, sev: severityError, msg: fmt.Sprintf("unknown status %q (must be one of DRAFT, APPROVED, IN_PROGRESS, DONE, FAILED, SUPERSEDED, ARCHIVED)", stateToken)})
		return fs
	}

	// Check for inline qualifier: the raw value should be exactly the state token.
	raw := strings.TrimSpace(m.statusRaw)
	if raw != stateToken {
		fs = append(fs, finding{line: ln, sev: severityError, msg: fmt.Sprintf("status %q has trailing inline qualifier %q (must be bare state token only)", stateToken, strings.TrimPrefix(raw, stateToken+" "))})
	}
	return fs
}

func validateSlugDeclared(m model) []finding {
	// Skip entirely if no slug (grandfathered)
	if m.slug == "" && m.slugLineCount == 0 {
		return nil
	}

	var fs []finding
	ln := m.slugLine
	if ln == 0 {
		ln = 1
	}

	if m.slugLineCount > 1 {
		fs = append(fs, finding{line: ln, sev: severityError, msg: fmt.Sprintf("duplicate ## Slug: lines (%d found, expected 1)", m.slugLineCount)})
		return fs
	}

	if !reSlugFormat.MatchString(m.slug) {
		fs = append(fs, finding{line: ln, sev: severityError, msg: fmt.Sprintf("slug %q is invalid (must match ^[A-Z][A-Z0-9]*$)", m.slug)})
	}
	return fs
}

func validateReqCovered(m model) []finding {
	// Build set of REQ IDs covered by TCs
	covered := make(map[string]bool)
	for _, tc := range m.tcs {
		covered[tc.reqRef] = true
	}

	var fs []finding
	for _, req := range m.reqs {
		if req.noTestPresent {
			if req.noTestReason == "" {
				fs = append(fs, finding{line: req.line, sev: severityError,
					msg: fmt.Sprintf("req %s has (no-test:) annotation with empty reason", req.id)})
			}
			// A REQ with valid no-test annotation is exempt regardless of whether it has a TC
			continue
		}
		if !covered[req.id] {
			fs = append(fs, finding{line: req.line, sev: severityError,
				msg: fmt.Sprintf("req %s has no test coverage (add a TC or annotate with (no-test: reason))", req.id)})
		}
	}
	return fs
}

func validateTaskHasFiles(m model) []finding {
	var fs []finding
	for _, t := range m.tasks {
		if t.execOnly {
			continue
		}
		if len(t.files) == 0 {
			fs = append(fs, finding{line: t.line, sev: severityError,
				msg: fmt.Sprintf("task %s has no files: entry", t.id)})
		}
	}
	return fs
}

// validateTestsRefValid checks that every TC ID referenced in a task's tests: entry
// is actually declared in the Test Plan. Skipped when the spec has no slug (grandfathered).
func validateTestsRefValid(m model) []finding {
	if m.slug == "" {
		return nil
	}
	tcSet := make(map[string]bool)
	for _, tc := range m.tcs {
		tcSet[tc.id] = true
	}
	var fs []finding
	for _, t := range m.tasks {
		for _, ref := range t.tests {
			if !tcSet[ref] {
				fs = append(fs, finding{line: t.line, sev: severityError,
					msg: fmt.Sprintf("task %s tests: references undeclared TC %q", t.id, ref)})
			}
		}
	}
	return fs
}

// validateTCReferenced checks that every TC declared in the Test Plan is referenced
// by at least one task's tests: entry. Skipped when the spec has no slug (grandfathered).
func validateTCReferenced(m model) []finding {
	if m.slug == "" {
		return nil
	}
	referenced := make(map[string]bool)
	for _, t := range m.tasks {
		for _, ref := range t.tests {
			referenced[ref] = true
		}
	}
	var fs []finding
	for _, tc := range m.tcs {
		if !referenced[tc.id] {
			fs = append(fs, finding{line: tc.line, sev: severityError,
				msg: fmt.Sprintf("TC %s is declared but referenced by no task's tests:", tc.id)})
		}
	}
	return fs
}

// excludedByPath returns true if the file path is excluded from the goProdTaskHasTests check.
func excludedByPath(path string) bool {
	// By basename: *_test.go
	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") {
		return true
	}
	// By exact path suffix (any OS path separator)
	excludedSuffixes := []string{
		"cmd/api/server.go",
		"cmd/api/container.go",
		"cmd/api/router.go",
	}
	normalized := filepath.ToSlash(path)
	for _, suffix := range excludedSuffixes {
		if normalized == suffix || strings.HasSuffix(normalized, "/"+suffix) {
			return true
		}
	}
	// By PATH containing /migration/
	if strings.Contains(normalized, "/migration/") {
		return true
	}
	return false
}

func validateGoProdTaskHasTests(m model) []finding {
	var fs []finding
	for _, t := range m.tasks {
		if len(t.tests) > 0 {
			continue
		}
		// Check if any file is a non-excluded production .go file
		hasNonExcludedGo := false
		for _, f := range t.files {
			if !strings.HasSuffix(f, ".go") {
				continue
			}
			if excludedByPath(f) {
				continue
			}
			hasNonExcludedGo = true
			break
		}
		if hasNonExcludedGo {
			fs = append(fs, finding{line: t.line, sev: severityWarn,
				msg: fmt.Sprintf("task %s has production .go file(s) but no tests: entry", t.id)})
		}
	}
	return fs
}

func validateDependsDAG(m model) []finding {
	// Build adjacency list
	adj := make(map[string][]string)
	for _, t := range m.tasks {
		adj[t.id] = t.depends
	}

	// Detect cycles using DFS with color marking
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	var cycleFound []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = gray
		for _, dep := range adj[node] {
			if color[dep] == gray {
				cycleFound = append(cycleFound, node+"->"+dep)
				return true
			}
			if color[dep] == white {
				if dfs(dep) {
					return true
				}
			}
		}
		color[node] = black
		return false
	}

	var fs []finding
	for _, t := range m.tasks {
		if color[t.id] == white {
			cycleFound = nil
			if dfs(t.id) {
				msg := fmt.Sprintf("dependency cycle detected involving task %s", t.id)
				if len(cycleFound) > 0 {
					msg = fmt.Sprintf("dependency cycle detected: %s", strings.Join(cycleFound, ", "))
				}
				fs = append(fs, finding{line: t.line, sev: severityError, msg: msg})
				// Reset to avoid duplicate reports
				for k := range color {
					color[k] = black
				}
			}
		}
	}
	return fs
}

func validateDependsExist(m model) []finding {
	taskIDs := make(map[string]bool)
	for _, t := range m.tasks {
		taskIDs[t.id] = true
	}

	var fs []finding
	for _, t := range m.tasks {
		for _, dep := range t.depends {
			if !taskIDs[dep] {
				fs = append(fs, finding{line: t.line, sev: severityError,
					msg: fmt.Sprintf("task %s depends on %s which does not exist", t.id, dep)})
			}
		}
	}
	return fs
}

func validateBatchMembership(m model) []finding {
	// Count how many batches each task appears in
	taskBatchCount := make(map[string]int)
	for _, b := range m.batches {
		for _, tid := range b.taskIDs {
			taskBatchCount[tid]++
		}
	}

	var fs []finding

	// Tasks that appear in 0 batches
	for _, t := range m.tasks {
		count := taskBatchCount[t.id]
		if count == 0 {
			fs = append(fs, finding{line: t.line, sev: severityError,
				msg: fmt.Sprintf("task %s is not in any batch", t.id)})
		} else if count > 1 {
			fs = append(fs, finding{line: t.line, sev: severityError,
				msg: fmt.Sprintf("task %s appears in %d batches (expected exactly 1)", t.id, count)})
		}
	}
	return fs
}

func validateBatchFileOverlap(m model) []finding {
	// Build task→files map
	taskFiles := make(map[string][]string)
	for _, t := range m.tasks {
		taskFiles[t.id] = t.files
	}

	var fs []finding
	for _, b := range m.batches {
		// Build shared-additive set for this batch
		saSet := make(map[string]bool)
		for _, f := range b.sharedAdditive {
			saSet[f] = true
		}

		// Collect file→first task that claimed it
		fileOwner := make(map[string]string)
		for _, tid := range b.taskIDs {
			for _, f := range taskFiles[tid] {
				if saSet[f] {
					continue
				}
				if owner, exists := fileOwner[f]; exists {
					fs = append(fs, finding{line: b.line, sev: severityError,
						msg: fmt.Sprintf("batch %d: tasks %s and %s both list file %q (not in shared-additive)", b.index, owner, tid, f)})
				} else {
					fileOwner[f] = tid
				}
			}
		}
	}
	return fs
}

func validateBatchOrder(m model) []finding {
	// Build task→batch index map
	taskBatchIdx := make(map[string]int)
	for _, b := range m.batches {
		for _, tid := range b.taskIDs {
			taskBatchIdx[tid] = b.index
		}
	}

	var fs []finding
	for _, t := range m.tasks {
		tBatch, tExists := taskBatchIdx[t.id]
		if !tExists {
			continue
		}
		for _, dep := range t.depends {
			depBatch, depExists := taskBatchIdx[dep]
			if !depExists {
				continue
			}
			// dep's batch must be strictly less than t's batch
			if depBatch >= tBatch {
				fs = append(fs, finding{line: t.line, sev: severityError,
					msg: fmt.Sprintf("task %s (batch %d) depends on %s (batch %d): dependency must be in an earlier batch", t.id, tBatch, dep, depBatch)})
			}
		}
	}
	return fs
}

// reSlugPrefix matches SLUG- prefix.
var reSlugTCID = regexp.MustCompile(`^([A-Z][A-Z0-9]*)-TC-([A-Z0-9]+)-(\d+)$`)
var reSlugREQID = regexp.MustCompile(`^([A-Z][A-Z0-9]*)-REQ-(\d+)$`)
var reLegacyTCID = regexp.MustCompile(`^TC-([A-Z0-9]+)-(\d+)$`)

func validateIDFormat(m model) []finding {
	var fs []finding

	if m.slug != "" {
		// Slug-prefixed mode: all REQ IDs must be SLUG-REQ-\d+, all TC IDs must be SLUG-TC-TYPE-\d{2,}
		for _, req := range m.reqs {
			match := reSlugREQID.FindStringSubmatch(req.id)
			if match == nil {
				fs = append(fs, finding{line: req.line, sev: severityError,
					msg: fmt.Sprintf("req id %q does not match expected format %s-REQ-N", req.id, m.slug)})
				continue
			}
			if match[1] != m.slug {
				fs = append(fs, finding{line: req.line, sev: severityError,
					msg: fmt.Sprintf("req id %q has wrong slug prefix %q (expected %s)", req.id, match[1], m.slug)})
			}
		}

		for _, tc := range m.tcs {
			match := reSlugTCID.FindStringSubmatch(tc.id)
			if match == nil {
				fs = append(fs, finding{line: tc.line, sev: severityError,
					msg: fmt.Sprintf("tc id %q does not match expected format %s-TC-TYPE-NN", tc.id, m.slug)})
				continue
			}
			prefix := match[1]
			tcType := match[2]
			digits := match[3]
			if prefix != m.slug {
				fs = append(fs, finding{line: tc.line, sev: severityError,
					msg: fmt.Sprintf("tc id %q has wrong slug prefix %q (expected %s)", tc.id, prefix, m.slug)})
			}
			if !registeredTCTypes[tcType] {
				fs = append(fs, finding{line: tc.line, sev: severityError,
					msg: fmt.Sprintf("tc id %q has unregistered type %q (must be one of D, UC, E2E, S, SH, CT)", tc.id, tcType)})
			}
			if len(digits) < 2 {
				fs = append(fs, finding{line: tc.line, sev: severityError,
					msg: fmt.Sprintf("tc id %q suffix %q must have at least 2 digits", tc.id, digits)})
			}
		}
	} else {
		// Legacy (no slug): only check that TC type is registered
		for _, tc := range m.tcs {
			// Try slug-prefixed pattern first
			if match := reSlugTCID.FindStringSubmatch(tc.id); match != nil {
				tcType := match[2]
				digits := match[3]
				if !registeredTCTypes[tcType] {
					fs = append(fs, finding{line: tc.line, sev: severityError,
						msg: fmt.Sprintf("tc id %q has unregistered type %q (must be one of D, UC, E2E, S, SH, CT)", tc.id, tcType)})
				}
				if len(digits) < 2 {
					fs = append(fs, finding{line: tc.line, sev: severityError,
						msg: fmt.Sprintf("tc id %q suffix %q must have at least 2 digits", tc.id, digits)})
				}
				continue
			}
			// Try legacy pattern
			if match := reLegacyTCID.FindStringSubmatch(tc.id); match != nil {
				tcType := match[1]
				digits := match[2]
				if !registeredTCTypes[tcType] {
					fs = append(fs, finding{line: tc.line, sev: severityError,
						msg: fmt.Sprintf("tc id %q has unregistered type %q (must be one of D, UC, E2E, S, SH, CT)", tc.id, tcType)})
				}
				if len(digits) < 2 {
					fs = append(fs, finding{line: tc.line, sev: severityError,
						msg: fmt.Sprintf("tc id %q suffix %q must have at least 2 digits", tc.id, digits)})
				}
				continue
			}
			// Doesn't match either pattern
			fs = append(fs, finding{line: tc.line, sev: severityError,
				msg: fmt.Sprintf("tc id %q does not match any recognized TC ID format", tc.id)})
		}
	}
	return fs
}

func validateIDUnique(m model) []finding {
	var fs []finding

	reqSeen := make(map[string]int)
	for _, req := range m.reqs {
		if prev, exists := reqSeen[req.id]; exists {
			fs = append(fs, finding{line: req.line, sev: severityError,
				msg: fmt.Sprintf("duplicate req id %q (first declared at line %d)", req.id, prev)})
		} else {
			reqSeen[req.id] = req.line
		}
	}

	tcSeen := make(map[string]int)
	for _, tc := range m.tcs {
		if prev, exists := tcSeen[tc.id]; exists {
			fs = append(fs, finding{line: tc.line, sev: severityError,
				msg: fmt.Sprintf("duplicate tc id %q (first declared at line %d)", tc.id, prev)})
		} else {
			tcSeen[tc.id] = tc.line
		}
	}
	return fs
}

func validateTCRefValid(m model) []finding {
	reqIDs := make(map[string]bool)
	for _, req := range m.reqs {
		reqIDs[req.id] = true
	}

	var fs []finding
	for _, tc := range m.tcs {
		if !reqIDs[tc.reqRef] {
			fs = append(fs, finding{line: tc.line, sev: severityError,
				msg: fmt.Sprintf("tc %s references req %q which does not exist", tc.id, tc.reqRef)})
		}
	}
	return fs
}

func validateCapabilityLinked(m model) []finding {
	// Only runs when slug is present
	if m.slug == "" {
		return nil
	}

	if reCapLink.MatchString(m.bodyStripped) {
		return nil
	}
	return []finding{{
		line: 1,
		sev:  severityWarn,
		msg:  "spec body does not reference any docs/capabilities/*.md file",
	}}
}
