package cmd

import (
	"github.com/drumilbhati/BackupDB/internal/config"
	"github.com/drumilbhati/BackupDB/internal/orchestrator"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate database credentials and connectivity",
	Long:  "Perform a connection preflight check to ensure the target database can be accessed",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			PrintResult(v.GetString("output.format"), nil, err, ExitInvalidInput)
		}

		if err := cfg.Validate("validate"); err != nil {
			PrintResult(cfg.Output.Format, nil, err, ExitInvalidInput)
		}

		orch := orchestrator.NewOrchestrator(cfg)
		err, code := orch.RunValidate()
		PrintResult(cfg.Output.Format, "Connection validation succeeded", err, code)
	},
}

func init() {
	validateCmd.Flags().String("db", "", "database type (postgres, mysql, mongodb, sqlite)")
	validateCmd.Flags().String("host", "", "database server host")
	validateCmd.Flags().Int("port", 0, "database server port")
	validateCmd.Flags().String("user", "", "database login username")
	validateCmd.Flags().String("password", "", "database login password")
	validateCmd.Flags().String("database", "", "database name or path to sqlite file")

	v.BindPFlag("db.type", validateCmd.Flags().Lookup("db"))
	v.BindPFlag("db.host", validateCmd.Flags().Lookup("host"))
	v.BindPFlag("db.port", validateCmd.Flags().Lookup("port"))
	v.BindPFlag("db.user", validateCmd.Flags().Lookup("user"))
	v.BindPFlag("db.password", validateCmd.Flags().Lookup("password"))
	v.BindPFlag("db.database", validateCmd.Flags().Lookup("database"))

	rootCmd.AddCommand(validateCmd)
}
