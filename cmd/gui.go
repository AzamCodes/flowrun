//go:build gui
package cmd

import (
	"flowrun/internal/gui"

	"github.com/spf13/cobra"
)

// guiCmd represents the gui command
var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the Flowrun desktop application",
	Long:  `Launches a graphical user interface for Flowrun.`,
	Run: func(cmd *cobra.Command, args []string) {
		app := gui.New()
		app.Run()
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
