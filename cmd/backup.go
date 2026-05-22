package cmd

import (
	"github.com/drumilbhati/BackupDB/internal/config"
	"github.com/drumilbhati/BackupDB/internal/orchestrator"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a database backup",
	Long:  "Create a full/incremental/differential backup of a database and persist it to a storage target",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			PrintResult(v.GetString("output.format"), nil, err, ExitInvalidInput)
		}

		if err := cfg.Validate("backup"); err != nil {
			PrintResult(cfg.Output.Format, nil, err, ExitInvalidInput)
		}

		orch := orchestrator.NewOrchestrator(cfg)
		outcome, err, code := orch.RunBackup()
		PrintResult(cfg.Output.Format, outcome, err, code)
	},
}

func init() {
	// DB specific
	backupCmd.Flags().String("db", "", "database type (postgres, mysql, mongodb, sqlite)")
	backupCmd.Flags().String("host", "", "database server host")
	backupCmd.Flags().Int("port", 0, "database server port")
	backupCmd.Flags().String("user", "", "database login username")
	backupCmd.Flags().String("password", "", "database login password")
	backupCmd.Flags().String("database", "", "database name or path to sqlite file")

	// Backup specific
	backupCmd.Flags().String("mode", "full", "backup mode (full, incremental, differential)")
	backupCmd.Flags().String("compress", "gzip", "compression algorithm (none, gzip, zstd)")
	backupCmd.Flags().Int("compression-level", 0, "compression level")

	// Storage specific
	backupCmd.Flags().String("storage", "", "storage type (local, s3, gcs, azure)")
	backupCmd.Flags().String("local-path", "", "local directory path for local storage")
	backupCmd.Flags().String("bucket", "", "bucket name for s3/gcs storage")
	backupCmd.Flags().String("prefix", "", "prefix path/key for storage storage")
	backupCmd.Flags().String("region", "", "AWS region for s3 storage")
	backupCmd.Flags().String("endpoint", "", "custom endpoint URL for s3 compatible storage")
	backupCmd.Flags().String("access-key", "", "access key ID for s3 storage")
	backupCmd.Flags().String("secret-key", "", "secret access key for s3 storage")
	backupCmd.Flags().String("container", "", "container name for azure storage")
	backupCmd.Flags().String("azure-account-name", "", "storage account name for azure storage")
	backupCmd.Flags().String("azure-account-key", "", "storage account key for azure storage")
	backupCmd.Flags().String("gcs-credentials-file", "", "path to GCS service account credentials JSON file")

	// Bindings
	v.BindPFlag("db.type", backupCmd.Flags().Lookup("db"))
	v.BindPFlag("db.host", backupCmd.Flags().Lookup("host"))
	v.BindPFlag("db.port", backupCmd.Flags().Lookup("port"))
	v.BindPFlag("db.user", backupCmd.Flags().Lookup("user"))
	v.BindPFlag("db.password", backupCmd.Flags().Lookup("password"))
	v.BindPFlag("db.database", backupCmd.Flags().Lookup("database"))

	v.BindPFlag("backup.mode", backupCmd.Flags().Lookup("mode"))
	v.BindPFlag("backup.compress", backupCmd.Flags().Lookup("compress"))
	v.BindPFlag("backup.compression_level", backupCmd.Flags().Lookup("compression-level"))

	v.BindPFlag("storage.type", backupCmd.Flags().Lookup("storage"))
	v.BindPFlag("storage.local_path", backupCmd.Flags().Lookup("local-path"))
	v.BindPFlag("storage.bucket", backupCmd.Flags().Lookup("bucket"))
	v.BindPFlag("storage.prefix", backupCmd.Flags().Lookup("prefix"))
	v.BindPFlag("storage.region", backupCmd.Flags().Lookup("region"))
	v.BindPFlag("storage.endpoint", backupCmd.Flags().Lookup("endpoint"))
	v.BindPFlag("storage.access_key", backupCmd.Flags().Lookup("access-key"))
	v.BindPFlag("storage.secret_key", backupCmd.Flags().Lookup("secret-key"))
	v.BindPFlag("storage.container", backupCmd.Flags().Lookup("container"))
	v.BindPFlag("storage.azure_account_name", backupCmd.Flags().Lookup("azure-account-name"))
	v.BindPFlag("storage.azure_account_key", backupCmd.Flags().Lookup("azure-account-key"))
	v.BindPFlag("storage.gcs_credentials_file", backupCmd.Flags().Lookup("gcs-credentials-file"))

	rootCmd.AddCommand(backupCmd)
}
