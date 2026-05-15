package learn

// Typed errors that classify failures into the exit-code contract from REQ-22
// (.specs/learning-loop-harness.md).
//
// Contract:
//
//	0 success
//	1 usage error  (unknown subcommand, invalid flag, validation failure)
//	2 runtime error (DB corruption, IO failure, panic-recovered)
//
// Callers wrap typed errors freely with fmt.Errorf("ctx: %w", err); exitCode
// uses errors.As so the wrapping chain is transparent.

import (
	"errors"
	"fmt"
)

// usageError signals a misuse of the CLI: unknown subcommand, invalid flag,
// validation failure on user input. Process exits with code 1.
type usageError struct {
	Msg string
	Err error // optional wrapped cause
}

// Error formats as "<msg>: <wrapped>" when Err is non-nil, otherwise "<msg>".
// Matches fmt.Errorf("%s: %w", msg, err) semantics for visual consistency.
func (e *usageError) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

// Unwrap exposes the wrapped cause for errors.Is/As chain traversal.
func (e *usageError) Unwrap() error { return e.Err }

// runtimeError signals an internal failure: DB corruption, IO error, recovered
// panic. Process exits with code 2.
type runtimeError struct {
	Msg string
	Err error
}

// Error formats as "<msg>: <wrapped>" when Err is non-nil, otherwise "<msg>".
func (e *runtimeError) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

// Unwrap exposes the wrapped cause for errors.Is/As chain traversal.
func (e *runtimeError) Unwrap() error { return e.Err }

// usagef returns a non-nil *usageError. It mirrors fmt.Errorf semantics: a
// %w verb (if present) wraps a cause that stays reachable via errors.Is/As.
//
// Implementation: we delegate to fmt.Errorf for verb handling and wrapping,
// then split the result into (Msg without trailing ": cause", Err = cause)
// so Error() does not double-print the cause.
func usagef(format string, args ...any) *usageError {
	msg, cause := splitWrapped(format, args...)
	return &usageError{Msg: msg, Err: cause}
}

// runtimef mirrors usagef but returns a *runtimeError.
func runtimef(format string, args ...any) *runtimeError {
	msg, cause := splitWrapped(format, args...)
	return &runtimeError{Msg: msg, Err: cause}
}

// splitWrapped formats via fmt.Errorf and returns (Msg, cause) such that
// reconstructing "Msg" + ": " + cause.Error() (when cause != nil) equals
// the original fmt.Errorf output. When the %w verb is not at the tail,
// the trailing-suffix invariant doesn't hold and we keep cause = nil,
// embedding the full formatted text in Msg — the caller still sees the
// fully formatted message, but loses unwrap reachability for that single
// non-canonical case (documented; not a supported pattern).
func splitWrapped(format string, args ...any) (string, error) {
	wrapped := fmt.Errorf(format, args...)
	cause := errors.Unwrap(wrapped)
	if cause == nil {
		return wrapped.Error(), nil
	}
	full := wrapped.Error()
	suffix := ": " + cause.Error()
	if len(full) > len(suffix) && full[len(full)-len(suffix):] == suffix {
		return full[:len(full)-len(suffix)], cause
	}
	return full, nil
}

// exitCode classifies an error into the REQ-22 contract:
//
//	nil           -> 0
//	*usageError   -> 1
//	*runtimeError -> 2
//	anything else -> 2 (fallback)
//
// Uses errors.As so wrapping via fmt.Errorf("ctx: %w", err) is transparent.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ue *usageError
	if errors.As(err, &ue) {
		return 1
	}
	var re *runtimeError
	if errors.As(err, &re) {
		return 2
	}
	return 2
}
