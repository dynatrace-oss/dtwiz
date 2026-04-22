package cmd

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the dtwiz version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("dtwiz %s\n", version.Version)
	},
}
