package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [filename]",
	Short: "Generate a workflow template",
	Long: `Generate a new workflow template file.
If no filename is provided, 'workflow.yaml' will be used by default.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filename := "workflow.yaml"
		if len(args) > 0 {
			filename = args[0]
			if filepath.Ext(filename) == "" {
				filename += ".yaml"
			}
		}

		if _, err := os.Stat(filename); err == nil {
			fmt.Printf("Error: file '%s' already exists\n", filename)
			os.Exit(1)
		}

		template := `name: My Workflow
env:
  APP_ENV: production

steps:
  - name: Check Go Version
    run: go version
    timeout: 10s

  - name: Run Tests
    run: go test ./...
    env:
      CGO_ENABLED: "0"
    retry: 2
    tags: [ci]

  - name: Build
    run: go build -v .
    timeout: 5m
`

		if err := os.WriteFile(filename, []byte(template), 0644); err != nil {
			fmt.Printf("Error creaing file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Workflow template created at '%s'\n", filename)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
