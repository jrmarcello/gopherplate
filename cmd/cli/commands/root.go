package commands

import "github.com/spf13/cobra"

var version = "dev" // set via ldflags at build time

var rootCmd = &cobra.Command{
	Use:   "gopherplate",
	Short: "Go microservice template scaffolding tool",
	Long: `Boilerplate CLI scaffolds new Go microservices and domains
following Clean Architecture patterns.

Commands:
  new          Create a new microservice from the template
  add domain   Add a new domain to an existing project
  version      Show CLI version`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(versionCmd)
}
