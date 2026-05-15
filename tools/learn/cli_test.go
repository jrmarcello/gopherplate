package learn

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TC-UC-30: unknown subcommand exits 1.
func TestRun_unknownSubcommand_exits1(t *testing.T) {
	var stderr bytes.Buffer
	got := runCLI([]string{"this-subcommand-does-not-exist"}, nil, &stderr)
	if got != 1 {
		t.Errorf("expected exit code 1 for unknown subcommand, got %d", got)
	}
}

// TC-UC-30a: --help on root command exits 0.
func TestRun_rootHelp_exits0(t *testing.T) {
	var stdout bytes.Buffer
	got := runCLI([]string{"--help"}, &stdout, nil)
	if got != 0 {
		t.Errorf("expected exit code 0 for --help, got %d", got)
	}
	out := stdout.String()
	if !strings.Contains(out, "learn") {
		t.Errorf("expected help output to mention 'learn', got: %q", out)
	}
}

// TC-UC-30b: --help on a registered subcommand exits 0.
// At bootstrap (TASK-1) no subcommands exist yet; this is a placeholder that
// becomes meaningful once subcommands are registered. We assert the command
// name is "learn" via the factory.
func TestNewRootCmd_useNameIsLearn(t *testing.T) {
	cmd := NewRootCmd()
	if cmd.Use != "learn" {
		t.Errorf("expected root command Use=\"learn\", got %q", cmd.Use)
	}
}

// Test_CobraRequiredFlag_exits_one verifies REQ-2 of fix-cobra-error-exit-codes:
// a subcommand that uses cobra.MarkFlagRequired must exit with code 1 when the
// flag is omitted (wrapped into *usageError by wrapCobraError's "required flag"
// match), not exit 2 via the fallback branch.
//
// No current learn subcommand uses MarkFlagRequired (they all validate inside
// their own RunE and return *usageError directly — exercised by TC-CT-11). This
// test installs a synthetic subcommand with cobra.MarkFlagRequired so the
// wrapCobraError "required flag" path is actually exercised and protected
// against mutation regressions.
func Test_CobraRequiredFlag_exits_one(t *testing.T) {
	t.Parallel()
	root := NewRootCmd()
	// SilenceUsage/SilenceErrors are NOT set on the subcommand — cobra
	// inherits both from root (NewRootCmd sets them). Setting them here
	// would create false parity with production subcommands and obscure
	// how silence propagation actually works.
	sub := &cobra.Command{
		Use: "synthetic",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	sub.Flags().String("required-flag", "", "a synthetic required flag")
	if markErr := sub.MarkFlagRequired("required-flag"); markErr != nil {
		t.Fatalf("MarkFlagRequired: %v", markErr)
	}
	root.AddCommand(sub)

	var stdout, stderr bytes.Buffer
	code := RunCmd(root, []string{"synthetic"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for missing required flag, got %d (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "required flag") {
		t.Errorf("expected stderr to contain \"required flag\", got: %q", stderr.String())
	}
}
