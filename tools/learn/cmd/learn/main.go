// Command learn is the entry point for the gopherplate learning loop tooling.
//
// Process exit code semantics (REQ-22):
//
//	0  success
//	1  usage error (unknown subcommand, invalid flag, validation failure)
//	2  runtime error (DB corruption, IO failure, etc.)
//
// Subcommands self-register into the learn package via init(); main only needs
// to call learn.RegisterAll on the root before dispatching.
package main

import (
	"os"

	"github.com/jrmarcello/gopherplate/tools/learn"
)

func main() {
	root := learn.NewRootCmd()
	learn.RegisterAll(root)
	os.Exit(learn.RunCmd(root, os.Args[1:], os.Stdout, os.Stderr))
}
