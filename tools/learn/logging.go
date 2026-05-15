package learn

// Structured logging for the `learn` CLI tooling (REQ-23). Every subcommand
// emits records in JSON to stderr with a "cmd" attribute identifying the
// originating subcommand. stdout is reserved for machine-readable command
// output (e.g. the JSON payload of `learn stats`).
//
// Wraps log/slog rather than introducing a third-party logger to stay aligned
// with the gopherplate "stdlib-first" posture.

import (
	"io"
	"log/slog"
)

// newJSONHandler returns a slog.Handler emitting JSON records to w. The handler
// uses slog defaults (keys: time, level, msg) and lets callers inject extra
// attributes via slog.With or slog.Logger.With.
//
// w is typically os.Stderr — stdout is reserved for command output. Tests can
// inject a *bytes.Buffer to capture records.
func newJSONHandler(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
}

// New returns a *slog.Logger pre-configured with a "cmd" attribute that is
// emitted on every record. cmdName should be the canonical subcommand name
// (e.g. "stats", "extract", "recall") so log filters can group events by
// subcommand.
//
// When cmdName is empty the attribute is still attached with an empty value;
// callers should always pass a non-empty name.
func newLogger(w io.Writer, cmdName string) *slog.Logger {
	return slog.New(newJSONHandler(w)).With("cmd", cmdName)
}
