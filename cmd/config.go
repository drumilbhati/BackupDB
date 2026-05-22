package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/drumilbhati/BackupDB/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show resolved configuration",
	Long:  "Load configuration from all sources, redact credentials, and output resolved settings",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(ExitInvalidInput)
		}

		redacted := cfg.Redact()

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(redacted); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding configuration: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
