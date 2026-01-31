package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"flowrun/internal/executor"
	"flowrun/internal/workflow"

	"github.com/spf13/cobra"
)

var (
	dryRun     bool
	targetSteps string
	targetTags  string
	envVars    []string
	envFile    string
	maxParallel int
	useCache    bool
	force       bool
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [file]",
	Short: "Execute a workflow from a YAML file",
	Long: `Execute a workflow defined in the specified YAML file.

Examples:
  flowrun run workflow.yaml
  flowrun run workflow.yaml --dry-run
  flowrun run workflow.yaml --steps "build,test"
  flowrun run workflow.yaml --env "GOOS=linux" --env "GOARCH=amd64" --env-file .env`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filename := args[0]

		// Load and validate workflow
		wf, err := workflow.Load(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading workflow: %v\n", err)
			os.Exit(2)
		}

		// Filter steps if requested
		if targetSteps != "" || targetTags != "" {
			var filtered []workflow.Step
			targetStepList := strings.Split(targetSteps, ",")
			targetTagList := strings.Split(targetTags, ",")

			for _, s := range wf.Steps {
				keep := false
				
				// 1. Check Name exact match
				if targetSteps != "" {
					for _, t := range targetStepList {
						if s.Name == strings.TrimSpace(t) {
							keep = true
							break
						}
					}
				}

				// 2. Check Tag match (if not already kept, or maybe OR logic?)
				// Let's implement OR logic: if matched name OR matched tag.
				// If both flags are empty, we keep all (handled outside).
				if targetTags != "" && !keep {
					for _, t := range targetTagList {
						t = strings.TrimSpace(t)
						for _, st := range s.Tags {
							if st == t {
								keep = true
								break
							}
						}
						if keep {
							break
						}
					}
				}
				
				if keep {
					filtered = append(filtered, s)
				}
			}

			if len(filtered) == 0 {
				fmt.Fprintln(os.Stderr, "Error: no steps matched the filters")
				os.Exit(1)
			}
			wf.Steps = filtered
		}

		// Load Env Variables
		envMap := make(map[string]string)
		
		// 1. Load from file
		if envFile != "" {
			fileEnv, err := loadEnvFile(envFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading env file: %v\n", err)
				os.Exit(1)
			}
			for k, v := range fileEnv {
				envMap[k] = v
			}
		}

		// 2. Load from flags (override file)
		for _, e := range envVars {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}

		// Execute workflow
		ctx := context.Background()
		opts := &executor.Options{
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			DryRun:      dryRun,
			Env:         envMap,
			MaxParallel: maxParallel,
			UseCache:    useCache,
			Force:       force,
		}

		if err := executor.Execute(ctx, wf, opts); err != nil {
			// Logger already handled error printing with more context, but exit code matters
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print steps without executing")
	runCmd.Flags().StringVar(&targetSteps, "steps", "", "Comma-separated list of steps to execute")
	runCmd.Flags().StringVar(&targetTags, "tags", "", "Comma-separated list of tags to execute")
	runCmd.Flags().StringSliceVarP(&envVars, "env", "e", nil, "Set environment variables (KEY=VALUE)")
	runCmd.Flags().StringVar(&envFile, "env-file", "", "Load environment variables from file")
	runCmd.Flags().IntVar(&maxParallel, "max-parallel", 0, "Max parallel steps (default: CPU cores)")
	runCmd.Flags().BoolVar(&useCache, "use-cache", false, "Use cached results if available")
	runCmd.Flags().BoolVar(&force, "force", false, "Force execution even if cached")
}

func loadEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env, scanner.Err()
}
