package cmd

import (
	"github.com/drumilbhati/BackupDB/internal/config"
	"github.com/drumilbhati/BackupDB/internal/orchestrator"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore from a backup",
	Long:  "Restore a database from a local or cloud-stored backup file",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			PrintResult(v.GetString("output.format"), nil, err, ExitInvalidInput)
		}

		if err := cfg.Validate("restore"); err != nil {
			PrintResult(cfg.Output.Format, nil, err, ExitInvalidInput)
		}

		orch := orchestrator.NewOrchestrator(cfg)
		outcome, err, code := orch.RunRestore()
		PrintResult(cfg.Output.Format, outcome, err, code)
	},
}

func init() {
	// DB specific (target database to restore to)
	restoreCmd.Flags().String("db", "", "database type (postgres, mysql, mongodb, sqlite)")
	restoreCmd.Flags().String("host", "", "database server host")
	restoreCmd.Flags().Int("port", 0, "database server port")
	restoreCmd.Flags().String("user", "", "database login username")
	restoreCmd.Flags().String("password", "", "database login password")
	restoreCmd.Flags().String("database", "", "database name or path to sqlite file")

	// Restore specific
	restoreCmd.Flags().String("backup-path", "", "path/URI to the backup file to restore from")
	restoreCmd.Flags().StringSlice("tables", nil, "comma-separated list of tables to selectively restore")
	restoreCmd.Flags().StringSlice("collections", nil, "comma-separated list of collections to selectively restore")

	// Storage specific (where backup-path is hosted, if not local)
	restoreCmd.Flags().String("storage", "local", "storage type where backup is hosted (local, s3, gcs, azure)")
	restoreCmd.Flags().String("local-path", "", "local directory path for local storage")
	restoreCmd.Flags().String("bucket", "", "bucket name for s3/gcs storage")
	restoreCmd.Flags().String("prefix", "", "prefix path/key for storage storage")
	restoreCmd.Flags().String("region", "", "AWS region for s3 storage")
	restoreCmd.Flags().String("endpoint", "", "custom endpoint URL for s3 compatible storage")
	restoreCmd.Flags().String("access-key", "", "access key ID for s3 storage")
	restoreCmd.Flags().String("secret-key", "", "secret access key for s3 storage")
	restoreCmd.Flags().String("container", "", "container name for azure storage")
	restoreCmd.Flags().String("azure-account-name", "", "storage account name for azure storage")
	restoreCmd.Flags().String("azure-account-key", "", "storage account key for azure storage")
	restoreCmd.Flags().String("gcs-credentials-file", "", "path to GCS service account credentials JSON file")

	// Bindings
	v.BindPFlag("db.type", restoreCmd.Flags().Lookup("db"))
	v.BindPFlag("db.host", restoreCmd.Flags().Lookup("host"))
	v.BindPFlag("db.port", restoreCmd.Flags().Lookup("port"))
	v.BindPFlag("db.user", restoreCmd.Flags().Lookup("user"))
	v.BindPFlag("db.password", restoreCmd.Flags().Lookup("password"))
	v.BindPFlag("db.database", restoreCmd.Flags().Lookup("database"))

	v.BindPFlag("restore.backup_path", restoreCmd.Flags().Lookup("backup-path"))
	v.BindPFlag("restore.tables", restoreCmd.Flags().Lookup("tables"))
	v.BindPFlag("restore.collections", restoreCmd.Flags().Lookup("collections"))

	v.BindPFlag("storage.type", restoreCmd.Flags().Lookup("storage"))
	v.BindPFlag("storage.local_path", restoreCmd.Flags().Lookup("local-path"))
	v.BindPFlag("storage.bucket", restoreCmd.Flags().Lookup("bucket"))
	v.BindPFlag("storage.prefix", restoreCmd.Flags().Lookup("prefix"))
	v.BindPFlag("storage.region", restoreCmd.Flags().Lookup("region"))
	v.BindPFlag("storage.endpoint", restoreCmd.Flags().Lookup("endpoint"))
	v.BindPFlag("storage.access_key", restoreCmd.Flags().Lookup("access-key"))
	v.BindPFlag("storage.secret_key", restoreCmd.Flags().Lookup("secret-key"))
	v.BindPFlag("storage.container", restoreCmd.Flags().Lookup("container"))
	v.BindPFlag("storage.azure_account_name", restoreCmd.Flags().Lookup("azure-account-name"))
	v.BindPFlag("storage.azure_account_key", restoreCmd.Flags().Lookup("azure-account-key"))
	v.BindPFlag("storage.gcs_credentials_file", restoreCmd.Flags().Lookup("gcs-credentials-file"))

	rootCmd.AddCommand(restoreCmd)
}
