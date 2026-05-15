package learn

import (
	"errors"
	"fmt"
	"testing"
)

// TC-D-15: a usageError wrapped via fmt.Errorf still classifies as exit 1.
func TestExitCode_wrappedUsageError_returns1(t *testing.T) {
	base := &usageError{Msg: "invalid flag"}
	wrapped := fmt.Errorf("ctx: %w", base)
	if got := exitCode(wrapped); got != 1 {
		t.Errorf("expected exit code 1 for wrapped usageError, got %d", got)
	}
}

// TC-D-16: a runtimeError wrapped via fmt.Errorf still classifies as exit 2.
func TestExitCode_wrappedRuntimeError_returns2(t *testing.T) {
	base := &runtimeError{Msg: "db corrupted"}
	wrapped := fmt.Errorf("ctx: %w", base)
	if got := exitCode(wrapped); got != 2 {
		t.Errorf("expected exit code 2 for wrapped runtimeError, got %d", got)
	}
}

// TC-D-17: an untyped plain error falls back to runtime exit 2.
func TestExitCode_plainError_returns2(t *testing.T) {
	if got := exitCode(errors.New("boom")); got != 2 {
		t.Errorf("expected exit code 2 fallback for plain error, got %d", got)
	}
}

func TestExitCode_nil_returns0(t *testing.T) {
	if got := exitCode(nil); got != 0 {
		t.Errorf("expected exit code 0 for nil error, got %d", got)
	}
}

func TestExitCode_directUsageError_returns1(t *testing.T) {
	if got := exitCode(&usageError{Msg: "x"}); got != 1 {
		t.Errorf("expected 1 for direct usageError, got %d", got)
	}
}

func TestExitCode_directRuntimeError_returns2(t *testing.T) {
	if got := exitCode(&runtimeError{Msg: "x"}); got != 2 {
		t.Errorf("expected 2 for direct runtimeError, got %d", got)
	}
}

func TestUsageError_errorFormat_withoutWrapped(t *testing.T) {
	e := &usageError{Msg: "unknown command"}
	if got := e.Error(); got != "unknown command" {
		t.Errorf("expected %q, got %q", "unknown command", got)
	}
}

func TestUsageError_errorFormat_withWrapped(t *testing.T) {
	cause := errors.New("bad input")
	e := &usageError{Msg: "validate", Err: cause}
	want := "validate: bad input"
	if got := e.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRuntimeError_errorFormat_withoutWrapped(t *testing.T) {
	e := &runtimeError{Msg: "io failure"}
	if got := e.Error(); got != "io failure" {
		t.Errorf("expected %q, got %q", "io failure", got)
	}
}

func TestRuntimeError_errorFormat_withWrapped(t *testing.T) {
	cause := errors.New("disk full")
	e := &runtimeError{Msg: "write", Err: cause}
	want := "write: disk full"
	if got := e.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestUsageError_unwrap(t *testing.T) {
	cause := errors.New("root")
	e := &usageError{Msg: "x", Err: cause}
	if !errors.Is(e, cause) {
		t.Errorf("expected errors.Is to find wrapped cause via Unwrap")
	}
}

func TestRuntimeError_unwrap(t *testing.T) {
	cause := errors.New("root")
	e := &runtimeError{Msg: "x", Err: cause}
	if !errors.Is(e, cause) {
		t.Errorf("expected errors.Is to find wrapped cause via Unwrap")
	}
}

func TestUsagef_nonNilOnEmptyFormat(t *testing.T) {
	e := usagef("")
	if e == nil {
		t.Fatalf("expected non-nil usageError even on empty format")
	}
	if e.Error() == "" {
		// An empty Msg with no wrapped cause is still a non-nil typed error,
		// which is what callers depend on for classification. We don't require
		// the message to be non-empty, just the pointer.
		_ = e
	}
}

func TestUsagef_formatsLikeErrorf(t *testing.T) {
	cause := errors.New("inner")
	e := usagef("ctx %d: %w", 42, cause)
	want := "ctx 42: inner"
	if got := e.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
	if !errors.Is(e, cause) {
		t.Errorf("expected wrapped cause to be reachable via errors.Is")
	}
}

func TestRuntimef_nonNilOnEmptyFormat(t *testing.T) {
	e := runtimef("")
	if e == nil {
		t.Fatalf("expected non-nil runtimeError even on empty format")
	}
}

func TestRuntimef_formatsLikeErrorf(t *testing.T) {
	cause := errors.New("inner")
	e := runtimef("ctx %d: %w", 7, cause)
	want := "ctx 7: inner"
	if got := e.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
	if !errors.Is(e, cause) {
		t.Errorf("expected wrapped cause to be reachable via errors.Is")
	}
}
