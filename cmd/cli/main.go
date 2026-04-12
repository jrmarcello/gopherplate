package main

import (
	"fmt"
	"os"

	"github.com/jrmarcello/gopherplate/cmd/cli/commands"
)

func main() {
	if execErr := commands.Execute(); execErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", execErr)
		os.Exit(1)
	}
}
