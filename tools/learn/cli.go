// cli.go wires the `learn` command tree and exposes Run as the testable
// entry point. Subcommands register themselves via init-time AddCommand calls
// from sibling files in package learn.
package learn

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// Run executes a bare root command with the given arguments and returns the
// process exit code per REQ-22:
//
//	0 success
//	1 usage error  (*usageError)
//	2 runtime error (*runtimeError, plus untyped fallback)
//
// stdout/stderr writers default to discard when nil — convenient for tests
// that don't care about output. Run does not register any subcommands; for
// the production entry point, build a root via NewRootCmd, attach subcommands
// (e.g. via RegisterAll), and call RunCmd.
func runCLI(args []string, stdout, stderr io.Writer) int {
	return RunCmd(NewRootCmd(), args, stdout, stderr)
}

// RunCmd executes a pre-built root command with the given arguments. Used by
// main(), which wires subcommands before calling RunCmd. Test code can still
// use Run for the bare root.
func RunCmd(root *cobra.Command, args []string, stdout, stderr io.Writer) int {
	root.SetArgs(args)
	if stdout != nil {
		root.SetOut(stdout)
	}
	if stderr != nil {
		root.SetErr(stderr)
	}
	if execErr := root.Execute(); execErr != nil {
		// Wrap cobra's untyped unknown-command + required-flag errors into
		// *usageError so exitCode returns 1 uniformly per REQ-4 of
		// .specs/refactor-tools-learn-flat-layout.md. See wrapCobraError
		// for the detection logic and double-wrap guard.
		execErr = wrapCobraError(execErr)
		// Cobra is set to SilenceErrors; we own the stderr emission.
		// After wrap, execErr.Error() returns the cobra message verbatim
		// (see learnerr.go: usagef("%s", err) → splitWrapped → Msg =
		// err.Error(), Err = nil → (*usageError).Error() = Msg).
		if stderr != nil {
			_, _ = fmt.Fprintln(stderr, "Error:", execErr)
		}
		return exitCode(execErr)
	}
	return 0
}

// wrapCobraError converts cobra's untyped errors (unknown command +
// missing-required-flag) into *usageError so exitCode returns 1 (matching
// REQ-4 of .specs/refactor-tools-learn-flat-layout.md). Errors already
// typed as *usageError or *runtimeError pass through unchanged — this
// prevents misclassifying a runtime error whose wrapped chain happens to
// mention "command" or "flag".
//
// errors.As is deliberate (not direct type-assert) because callers may
// wrap typed errors via fmt.Errorf("...: %w", typedErr) — we want to
// detect the typed error anywhere in the wrap chain, not just at the top.
//
// String-match patterns are taken from cobra v1's internal messages and
// MUST be re-verified on any major cobra version upgrade. Current cobra
// emits: "unknown command \"X\" for \"Y\"" (Command.Find) and "required
// flag(s) \"X\", ... not set" (Command.ValidateRequiredFlags).
func wrapCobraError(err error) error {
	if err == nil {
		return nil
	}
	var ue *usageError
	var re *runtimeError
	if errors.As(err, &ue) || errors.As(err, &re) {
		return err
	}
	msg := err.Error()
	if strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "required flag") {
		return usagef("%s", err)
	}
	return err
}

// NewRootCmd returns the root cobra command. Exported so subcommand files
// (and tests) can attach to it without going through Run.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "learn",
		Short: "Learning loop tooling for the gopherplate harness",
		Long: `learn powers the gopherplate harness's closed-loop learning system:
pattern extraction over execution logs / transcripts / git, FTS5-backed
retrieval, periodic nudge with TTL-based deprecation, and explicit decision
tracking for skill/memory refinement.`,
		// RunE rejects positional args explicitly. Cobra's default "no Run
		// function" behavior falls back to printing help with exit 0 even
		// when Args=NoArgs is set, which masks unknown-subcommand errors at
		// bootstrap (before any subcommand is registered).
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usagef("unknown command %q for %q", args[0], cmd.CommandPath())
			}
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// Wrap cobra-emitted flag-parse errors (unknown flag, bad flag value)
	// into *usageError so exitCode returns 1. Without this hook, the
	// unwrapped error falls through exitCode's fallback to exit 2,
	// violating REQ-4 of the parent refactor spec.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usagef("%s", err)
	})
	return root
}
