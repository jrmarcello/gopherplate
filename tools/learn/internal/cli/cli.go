// Package cli wires the `learn` command tree and exposes Run as the testable
// entry point. Subcommands register themselves via init-time AddCommand calls
// from sibling packages.
package cli

import (
	"fmt"
	"io"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/spf13/cobra"
)

// Run executes a bare root command with the given arguments and returns the
// process exit code per REQ-22:
//
//	0 success
//	1 usage error  (*learnerr.UsageError)
//	2 runtime error (*learnerr.RuntimeError, plus untyped fallback)
//
// stdout/stderr writers default to discard when nil — convenient for tests
// that don't care about output. Run does not register any subcommands; for
// the production entry point, build a root via NewRootCmd, attach subcommands
// (e.g. via cmd.RegisterAll), and call RunCmd.
func Run(args []string, stdout, stderr io.Writer) int {
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
		// Cobra is set to SilenceErrors; we own the stderr emission.
		if stderr != nil {
			fmt.Fprintln(stderr, "Error:", execErr)
		}
		return learnerr.ExitCode(execErr)
	}
	return 0
}

// NewRootCmd returns the root cobra command. Exported so subcommand packages
// (and tests) can attach to it without going through Run.
func NewRootCmd() *cobra.Command {
	return &cobra.Command{
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
				return learnerr.Usagef("unknown command %q for %q", args[0], cmd.CommandPath())
			}
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}
