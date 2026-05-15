package learn

import (
	"bytes"
	"strings"
	"testing"
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
