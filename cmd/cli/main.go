package main

import (
	"fmt"
	"os"

	"github.com/jrmarcello/go-boilerplate/cmd/cli/commands"
)

func main() {
	if execErr := commands.Execute(); execErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", execErr)
		os.Exit(1)
	}
}
