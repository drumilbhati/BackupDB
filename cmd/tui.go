package cmd

import (
	"github.com/drumilbhati/BackupDB/internal/config"
	"github.com/drumilbhati/BackupDB/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive terminal UI",
	Long:  "Launch a Bubble Tea-based terminal UI for guided backup, restore, validate, config, and version workflows",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig(v, cfgFile)
		if err != nil {
			return err
		}

		return tui.Run(cfg, cfgFile)
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
