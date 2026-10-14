package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the dtwiz version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("dtwiz version: %s\n", version.Version)
		fmt.Printf("commit: %s\n", version.Commit)
		fmt.Printf("built: %s\n", version.Date)
		fireCompletedEvent(cmd, nil)
	},
}
