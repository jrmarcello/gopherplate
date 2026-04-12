package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show CLI version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("gopherplate %s\n", version)
	},
}
