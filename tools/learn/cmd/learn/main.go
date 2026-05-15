// Command learn is the entry point for the gopherplate learning loop tooling.
//
// Process exit code semantics (REQ-22):
//
//	0  success
//	1  usage error (unknown subcommand, invalid flag, validation failure)
//	2  runtime error (DB corruption, IO failure, etc.)
//
// Subcommands self-register into the internal/cmd package via init(); main
// only needs to call cmd.RegisterAll on the root before dispatching.
package main

import (
	"os"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/cli"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/cmd"
)

func main() {
	root := cli.NewRootCmd()
	cmd.RegisterAll(root)
	os.Exit(cli.RunCmd(root, os.Args[1:], os.Stdout, os.Stderr))
}
