package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the utility version",
	Long:  "Print the current version of the Database Backup and Restore Utility",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("backupdb version 1.0.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
