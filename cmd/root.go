package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	rootCmd.PersistentFlags().String("config", "", "config file path")
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug|info|warn|error)")
	rootCmd.PersistentFlags().String("output", "text", "output format (text|json)")

	// Bind flags to Viper so env vars and config file also work
	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	// Env var prefix: BACKUPDB_LOG_LEVEL, BACKUPDB_OUTPUT, etc.
	viper.SetEnvPrefix("BACKUPDB")
	viper.AutomaticEnv()
}
