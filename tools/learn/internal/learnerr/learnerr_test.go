package learnerr

import (
	"errors"
	"fmt"
	"testing"
)

// TC-D-15: a UsageError wrapped via fmt.Errorf still classifies as exit 1.
func TestExitCode_wrappedUsageError_returns1(t *testing.T) {
	base := &UsageError{Msg: "invalid flag"}
	wrapped := fmt.Errorf("ctx: %w", base)
	if got := ExitCode(wrapped); got != 1 {
		t.Errorf("expected exit code 1 for wrapped UsageError, got %d", got)
	}
}

// TC-D-16: a RuntimeError wrapped via fmt.Errorf still classifies as exit 2.
func TestExitCode_wrappedRuntimeError_returns2(t *testing.T) {
	base := &RuntimeError{Msg: "db corrupted"}
	wrapped := fmt.Errorf("ctx: %w", base)
	if got := ExitCode(wrapped); got != 2 {
		t.Errorf("expected exit code 2 for wrapped RuntimeError, got %d", got)
	}
}

// TC-D-17: an untyped plain error falls back to runtime exit 2.
func TestExitCode_plainError_returns2(t *testing.T) {
	if got := ExitCode(errors.New("boom")); got != 2 {
		t.Errorf("expected exit code 2 fallback for plain error, got %d", got)
	}
}

func TestExitCode_nil_returns0(t *testing.T) {
	if got := ExitCode(nil); got != 0 {
		t.Errorf("expected exit code 0 for nil error, got %d", got)
	}
}

func TestExitCode_directUsageError_returns1(t *testing.T) {
	if got := ExitCode(&UsageError{Msg: "x"}); got != 1 {
		t.Errorf("expected 1 for direct UsageError, got %d", got)
	}
}

func TestExitCode_directRuntimeError_returns2(t *testing.T) {
	if got := ExitCode(&RuntimeError{Msg: "x"}); got != 2 {
		t.Errorf("expected 2 for direct RuntimeError, got %d", got)
	}
}

func TestUsageError_errorFormat_withoutWrapped(t *testing.T) {
	e := &UsageError{Msg: "unknown command"}
	if got := e.Error(); got != "unknown command" {
		t.Errorf("expected %q, got %q", "unknown command", got)
	}
}

func TestUsageError_errorFormat_withWrapped(t *testing.T) {
	cause := errors.New("bad input")
	e := &UsageError{Msg: "validate", Err: cause}
	want := "validate: bad input"
	if got := e.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRuntimeError_errorFormat_withoutWrapped(t *testing.T) {
	e := &RuntimeError{Msg: "io failure"}
	if got := e.Error(); got != "io failure" {
		t.Errorf("expected %q, got %q", "io failure", got)
	}
}

func TestRuntimeError_errorFormat_withWrapped(t *testing.T) {
	cause := errors.New("disk full")
	e := &RuntimeError{Msg: "write", Err: cause}
	want := "write: disk full"
	if got := e.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestUsageError_unwrap(t *testing.T) {
	cause := errors.New("root")
	e := &UsageError{Msg: "x", Err: cause}
	if !errors.Is(e, cause) {
		t.Errorf("expected errors.Is to find wrapped cause via Unwrap")
	}
}

func TestRuntimeError_unwrap(t *testing.T) {
	cause := errors.New("root")
	e := &RuntimeError{Msg: "x", Err: cause}
	if !errors.Is(e, cause) {
		t.Errorf("expected errors.Is to find wrapped cause via Unwrap")
	}
}

func TestUsagef_nonNilOnEmptyFormat(t *testing.T) {
	e := Usagef("")
	if e == nil {
		t.Fatalf("expected non-nil UsageError even on empty format")
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
	e := Usagef("ctx %d: %w", 42, cause)
	want := "ctx 42: inner"
	if got := e.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
	if !errors.Is(e, cause) {
		t.Errorf("expected wrapped cause to be reachable via errors.Is")
	}
}

func TestRuntimef_nonNilOnEmptyFormat(t *testing.T) {
	e := Runtimef("")
	if e == nil {
		t.Fatalf("expected non-nil RuntimeError even on empty format")
	}
}

func TestRuntimef_formatsLikeErrorf(t *testing.T) {
	cause := errors.New("inner")
	e := Runtimef("ctx %d: %w", 7, cause)
	want := "ctx 7: inner"
	if got := e.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
	if !errors.Is(e, cause) {
		t.Errorf("expected wrapped cause to be reachable via errors.Is")
	}
}
