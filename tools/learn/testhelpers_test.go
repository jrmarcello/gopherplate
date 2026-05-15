package learn

import "testing"

// newTestSanitizer builds the canonical sanitizer used across the ingest_*
// parser tests. It pulls the default secret patterns so AKIA fixtures get
// redacted exactly as they would in production. A construction failure is
// fatal for the test process because every TC depends on a working sanitizer.
//
// Consolidated from the per-package variants that previously lived in
// internal/ingest/{spec,git,transcript,memory}/parser_test.go.
func newTestSanitizer(t *testing.T) *sanitizer {
	t.Helper()
	pats, buildErr := builtinSanitizePatterns()
	if buildErr != nil {
		t.Fatalf("builtinSanitizePatterns: %v", buildErr)
	}
	return newSanitizer(pats)
}
