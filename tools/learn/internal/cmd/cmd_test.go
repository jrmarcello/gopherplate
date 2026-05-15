package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// snapshotRegistrars saves the current registrars slice and restores it on
// test cleanup so test-added registrars don't leak into other tests in the
// same process.
func snapshotRegistrars(t *testing.T) {
	t.Helper()
	saved := append([]func(*cobra.Command){}, registrars...)
	t.Cleanup(func() { registrars = saved })
}

// TestRegister_AppendsToRegistrars verifies register() adds a function to the
// package-level slice.
func TestRegister_AppendsToRegistrars(t *testing.T) {
	snapshotRegistrars(t)
	before := len(registrars)
	register(func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{Use: "dummytest"})
	})
	if got := len(registrars); got != before+1 {
		t.Fatalf("expected registrars len %d, got %d", before+1, got)
	}
}

// TestRegisterAll_AttachesAllRegistrars verifies RegisterAll wires every
// registered subcommand onto the supplied root.
func TestRegisterAll_AttachesAllRegistrars(t *testing.T) {
	snapshotRegistrars(t)
	register(func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{Use: "dummytest"})
	})

	root := &cobra.Command{Use: "learn"}
	RegisterAll(root)

	found := false
	for _, c := range root.Commands() {
		if c.Use == "dummytest" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected RegisterAll to attach 'dummytest' subcommand; got %v", root.Commands())
	}
}
