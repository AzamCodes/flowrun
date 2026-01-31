package cmd

import (
	"os"

	"flowrun/internal/logger"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "flowrun",
	Short: "A CLI/Production-grade Go tool for running workflows",
	Long: `flowrun is a CLI tool designed to execute workflows defined in YAML files.
It is built with simplicity and reliability in mind.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initLogger()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Root flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "INFO", "Log level (DEBUG, INFO, WARN, ERROR)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Log to file")
	rootCmd.PersistentFlags().BoolVar(&jsonLog, "json", false, "Log in JSON format") // Added JSON flag
}

var (
	logLevel string
	logFile  string
	jsonLog  bool
)

func initLogger() {
	cfg := logger.Config{
		Level:  logLevel,
		Format: "text",
	}
	if jsonLog {
		cfg.Format = "json"
	}
	
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			// If we can't open the log file, start with default logger to print error
			logger.Setup(logger.Config{Level: "INFO", Format: "text", Output: os.Stderr})
			logger.Error("Failed to open log file", "file", logFile, "error", err)
			os.Exit(1)
		}
		cfg.Output = f
	} else {
		cfg.Output = os.Stderr
	}

	logger.Setup(cfg)
}
