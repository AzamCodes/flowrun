package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version can be set at build time using -ldflags
var Version = "dev"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of flowrun",
	Long:  `All software has versions. This is flowrun's.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("flowrun version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
