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
		fmt.Printf("dtwiz %s\n", version.Version)
	},
}
