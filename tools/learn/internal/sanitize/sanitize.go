// Package sanitize redacts secret material from prompt/response text before
// it is persisted by the learning-loop store (REQ-4 of the
// learning-loop-harness spec).
//
// The package is intentionally tiny and dependency-free apart from the
// project's config package (which owns the canonical regex specs). The
// Sanitize function is pure: it operates on the input string only and never
// touches the filesystem, the network, or any global state. Callers are
// responsible for invoking Sanitize on any string they intend to write to
// disk or send over the wire — there is no implicit IO hook here.
package sanitize

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
)

// Pattern is a compiled secret-matching rule. Kind is the semantic label
// emitted inside the redaction marker (e.g. "aws_key" → "<REDACTED:aws_key>").
type Pattern struct {
	Kind string
	Re   *regexp.Regexp
}

// Sanitizer applies an ordered list of Patterns to incoming text. Patterns
// are applied in the order supplied at construction; the first match wins
// for overlapping ranges (a consequence of replacing one pattern's matches
// before the next pattern sees the text).
type Sanitizer struct {
	patterns []Pattern
}

// New builds a Sanitizer over pre-compiled patterns. The input slice is
// shallow-copied so later mutations by the caller do not affect the
// Sanitizer.
func New(patterns []Pattern) *Sanitizer {
	cp := make([]Pattern, len(patterns))
	copy(cp, patterns)
	return &Sanitizer{patterns: cp}
}

// NewFromConfig builds a Sanitizer from the raw {Kind, Pattern} specs that
// live in the project config. Patterns that look line-anchored (start with
// '^' or end with '$') are automatically promoted to multi-line mode by
// prefixing "(?m)" so they can match per-line inside multi-line payloads
// such as .env files — REQ-4 explicitly requires env_value matching to work
// in that context.
//
// On compilation failure of any pattern, NewFromConfig returns an error that
// names the offending Kind.
func NewFromConfig(specs []config.SecretPattern) (*Sanitizer, error) {
	pats := make([]Pattern, 0, len(specs))
	for _, spec := range specs {
		raw := spec.Pattern
		if needsMultiline(raw) && !strings.HasPrefix(raw, "(?m)") {
			raw = "(?m)" + raw
		}
		re, compileErr := regexp.Compile(raw)
		if compileErr != nil {
			return nil, fmt.Errorf("sanitize: compile pattern %q (kind=%s): %w", spec.Pattern, spec.Kind, compileErr)
		}
		pats = append(pats, Pattern{Kind: spec.Kind, Re: re})
	}
	return &Sanitizer{patterns: pats}, nil
}

// needsMultiline reports whether a pattern uses line anchors and should be
// compiled with the multi-line flag so it can match per-line in larger
// payloads.
func needsMultiline(raw string) bool {
	if strings.HasPrefix(raw, "^") {
		return true
	}
	// Detect a trailing '$' that is not escaped.
	if strings.HasSuffix(raw, "$") && !strings.HasSuffix(raw, `\$`) {
		return true
	}
	return false
}

// Sanitize replaces every match of any configured pattern in in with the
// literal token "<REDACTED:<kind>>". Patterns are applied in declaration
// order; once a substring has been replaced by an earlier pattern it cannot
// be re-matched by a later one.
//
// Sanitize is a pure function — it never reads from or writes to the
// filesystem, never logs, and never holds the input beyond the call. The
// "sanitize before any write" guarantee required by REQ-4 is therefore the
// responsibility of every caller: invoke Sanitize before passing data to
// any store, log, or IO layer.
func (s *Sanitizer) Sanitize(in string) string {
	if in == "" || len(s.patterns) == 0 {
		return in
	}
	out := in
	for _, p := range s.patterns {
		marker := "<REDACTED:" + p.Kind + ">"
		out = p.Re.ReplaceAllString(out, marker)
	}
	return out
}

// SanitizeBytes is the []byte variant of Sanitize and shares its semantics
// (purity, ordering, redaction marker format). It allocates a new slice and
// never mutates the input.
func (s *Sanitizer) SanitizeBytes(in []byte) []byte {
	if len(in) == 0 || len(s.patterns) == 0 {
		out := make([]byte, len(in))
		copy(out, in)
		return out
	}
	out := in
	for _, p := range s.patterns {
		marker := []byte("<REDACTED:" + p.Kind + ">")
		out = p.Re.ReplaceAll(out, marker)
	}
	// Guarantee a fresh backing array when no substitution occurred so the
	// caller cannot accidentally mutate our input alias.
	if &out[0] == &in[0] {
		cp := make([]byte, len(out))
		copy(cp, out)
		return cp
	}
	return out
}
