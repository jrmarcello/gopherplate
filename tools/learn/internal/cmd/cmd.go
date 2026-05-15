// Package cmd contains every learn subcommand. Each sibling file registers
// itself via init() into a package-level slice, and RegisterAll wires them
// onto a root cobra.Command at process start.
//
// This decouples subcommand authoring from the dispatcher: adding a new
// subcommand is a matter of dropping a file in this package that calls
// register() from its init(). main() only needs to call RegisterAll once.
package cmd

import "github.com/spf13/cobra"

// registrars holds every subcommand registrar added via register(). Order of
// entries follows Go's file-load order, which is deterministic within a single
// build but not guaranteed across versions; subcommands MUST NOT rely on
// sibling subcommand registration order.
var registrars []func(*cobra.Command)

// register is package-private; sibling files call it from init() to wire
// their subcommand factory onto the root command at RegisterAll time.
func register(reg func(*cobra.Command)) {
	registrars = append(registrars, reg)
}

// RegisterAll attaches every subcommand registered through init() onto root.
// Callers should invoke this exactly once, immediately after building the
// root command and before executing it.
func RegisterAll(root *cobra.Command) {
	for _, reg := range registrars {
		reg(root)
	}
}
