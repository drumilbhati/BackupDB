package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Exit codes
const (
	ExitSuccess           = 0
	ExitInvalidInput      = 2
	ExitConnectionFailure = 3
	ExitBackupFailure     = 4
	ExitRestoreFailure    = 5
	ExitStorageFailure    = 6
)

var (
	cfgFile string
	v       = viper.New()
)

var rootCmd = &cobra.Command{
	Use:   "backupdb",
	Short: "Database backup and restore utility",
	Long:  "Back up and restore PostgreSQL, MySQL, MongoDB, and SQLite databases",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./backupdb.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "logging level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-file", "", "log file path (defaults to stdout)")
	rootCmd.PersistentFlags().String("slack-webhook", "", "Slack webhook URL for notifications")
	rootCmd.PersistentFlags().String("output", "text", "output format (text, json)")
	rootCmd.PersistentFlags().String("catalog-path", "./.backupdb/catalog.json", "catalog file path for backup chain metadata")

	v.BindPFlag("logging.level", rootCmd.PersistentFlags().Lookup("log-level"))
	v.BindPFlag("logging.file", rootCmd.PersistentFlags().Lookup("log-file"))
	v.BindPFlag("notifications.slack_webhook", rootCmd.PersistentFlags().Lookup("slack-webhook"))
	v.BindPFlag("output.format", rootCmd.PersistentFlags().Lookup("output"))
	v.BindPFlag("catalog.path", rootCmd.PersistentFlags().Lookup("catalog-path"))
}

// PrintResult outputs the command outcome in the requested format (text or JSON) and exits with the appropriate code.
func PrintResult(format string, outcome interface{}, err error, exitCode int) {
	if format == "json" {
		out := make(map[string]interface{})
		if err != nil {
			out["status"] = "error"
			out["error"] = err.Error()
			out["code"] = exitCode
		} else {
			out["status"] = "success"
			if outcome != nil {
				// merge fields or nest outcome
				out["data"] = outcome
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else if outcome != nil {
			switch val := outcome.(type) {
			case string:
				fmt.Println(val)
			default:
				fmt.Printf("Success: %+v\n", outcome)
			}
		} else {
			fmt.Println("Success")
		}
	}
	os.Exit(exitCode)
}
