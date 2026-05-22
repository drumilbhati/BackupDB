package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	DB            DBConfig            `mapstructure:"db"`
	Backup        BackupConfig        `mapstructure:"backup"`
	Storage       StorageConfig       `mapstructure:"storage"`
	Restore       RestoreConfig       `mapstructure:"restore"`
	Logging       LoggingConfig       `mapstructure:"logging"`
	Notifications NotificationsConfig `mapstructure:"notifications"`
	Output        OutputConfig        `mapstructure:"output"`
}

type DBConfig struct {
	Type     string `mapstructure:"type"` // postgres, mysql, mongodb, sqlite
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type BackupConfig struct {
	Mode             string `mapstructure:"mode"`              // full, incremental, differential
	Compress         string `mapstructure:"compress"`          // none, gzip, zstd
	CompressionLevel int    `mapstructure:"compression_level"` // optional level override
}

type StorageConfig struct {
	Type               string `mapstructure:"type"` // local, s3, gcs, azure
	LocalPath          string `mapstructure:"local_path"`
	Bucket             string `mapstructure:"bucket"`    // s3, gcs
	Prefix             string `mapstructure:"prefix"`    // s3, gcs, azure
	Region             string `mapstructure:"region"`    // s3
	Endpoint           string `mapstructure:"endpoint"`  // s3 compatible
	AccessKey          string `mapstructure:"access_key"`
	SecretKey          string `mapstructure:"secret_key"`
	Container          string `mapstructure:"container"` // azure
	AzureAccountName   string `mapstructure:"azure_account_name"`
	AzureAccountKey    string `mapstructure:"azure_account_key"`
	GCSCredentialsFile string `mapstructure:"gcs_credentials_file"`
}

type RestoreConfig struct {
	BackupPath  string   `mapstructure:"backup_path"`
	Tables      []string `mapstructure:"tables"`
	Collections []string `mapstructure:"collections"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

type NotificationsConfig struct {
	SlackWebhook string `mapstructure:"slack_webhook"`
}

type OutputConfig struct {
	Format string `mapstructure:"format"` // text, json
}

// LoadConfig resolves configuration from the config file, environment variables, defaults, and CLI flags.
func LoadConfig(cfgFile string) (*Config, error) {
	v := viper.New()

	// 1. Set Defaults
	setDefaults(v)

	// 2. Set Env Precedence & Bindings
	v.SetEnvPrefix("BACKUPDB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnvVars(v)

	// 3. Load Config File
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.AddConfigPath(".")
		v.SetConfigName("backupdb")
		v.SetConfigType("yaml")
	}

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) && cfgFile != "" {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Expand environment variables (e.g. ${BACKUPDB_PASSWORD}) in strings
	expandEnvInConfig(&cfg)

	return &cfg, nil
}

func bindEnvVars(v *viper.Viper) {
	v.BindEnv("db.type", "BACKUPDB_DB_TYPE")
	v.BindEnv("db.host", "BACKUPDB_DB_HOST")
	v.BindEnv("db.port", "BACKUPDB_DB_PORT")
	v.BindEnv("db.user", "BACKUPDB_DB_USER")
	v.BindEnv("db.password", "BACKUPDB_DB_PASSWORD")
	v.BindEnv("db.database", "BACKUPDB_DB_DATABASE")

	v.BindEnv("backup.mode", "BACKUPDB_BACKUP_MODE")
	v.BindEnv("backup.compress", "BACKUPDB_BACKUP_COMPRESS")
	v.BindEnv("backup.compression_level", "BACKUPDB_BACKUP_COMPRESSION_LEVEL")

	v.BindEnv("storage.type", "BACKUPDB_STORAGE_TYPE")
	v.BindEnv("storage.local_path", "BACKUPDB_STORAGE_LOCAL_PATH")
	v.BindEnv("storage.bucket", "BACKUPDB_STORAGE_BUCKET")
	v.BindEnv("storage.prefix", "BACKUPDB_STORAGE_PREFIX")
	v.BindEnv("storage.region", "BACKUPDB_STORAGE_REGION")
	v.BindEnv("storage.endpoint", "BACKUPDB_STORAGE_ENDPOINT")
	v.BindEnv("storage.access_key", "BACKUPDB_STORAGE_ACCESS_KEY")
	v.BindEnv("storage.secret_key", "BACKUPDB_STORAGE_SECRET_KEY")
	v.BindEnv("storage.container", "BACKUPDB_STORAGE_CONTAINER")
	v.BindEnv("storage.azure_account_name", "BACKUPDB_STORAGE_AZURE_ACCOUNT_NAME")
	v.BindEnv("storage.azure_account_key", "BACKUPDB_STORAGE_AZURE_ACCOUNT_KEY")
	v.BindEnv("storage.gcs_credentials_file", "BACKUPDB_STORAGE_GCS_CREDENTIALS_FILE")

	v.BindEnv("restore.backup_path", "BACKUPDB_RESTORE_BACKUP_PATH")
	v.BindEnv("restore.tables", "BACKUPDB_RESTORE_TABLES")
	v.BindEnv("restore.collections", "BACKUPDB_RESTORE_COLLECTIONS")

	v.BindEnv("logging.level", "BACKUPDB_LOGGING_LEVEL")
	v.BindEnv("logging.file", "BACKUPDB_LOGGING_FILE")

	v.BindEnv("notifications.slack_webhook", "BACKUPDB_NOTIFICATIONS_SLACK_WEBHOOK")

	v.BindEnv("output.format", "BACKUPDB_OUTPUT_FORMAT")
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("db.port", 0)
	v.SetDefault("backup.mode", "full")
	v.SetDefault("backup.compress", "gzip")
	v.SetDefault("logging.level", "info")
	v.SetDefault("output.format", "text")
}

func expandEnvInConfig(cfg *Config) {
	cfg.DB.Host = os.ExpandEnv(cfg.DB.Host)
	cfg.DB.User = os.ExpandEnv(cfg.DB.User)
	cfg.DB.Password = os.ExpandEnv(cfg.DB.Password)
	cfg.DB.Database = os.ExpandEnv(cfg.DB.Database)

	cfg.Storage.LocalPath = os.ExpandEnv(cfg.Storage.LocalPath)
	cfg.Storage.Bucket = os.ExpandEnv(cfg.Storage.Bucket)
	cfg.Storage.Prefix = os.ExpandEnv(cfg.Storage.Prefix)
	cfg.Storage.Region = os.ExpandEnv(cfg.Storage.Region)
	cfg.Storage.Endpoint = os.ExpandEnv(cfg.Storage.Endpoint)
	cfg.Storage.AccessKey = os.ExpandEnv(cfg.Storage.AccessKey)
	cfg.Storage.SecretKey = os.ExpandEnv(cfg.Storage.SecretKey)
	cfg.Storage.Container = os.ExpandEnv(cfg.Storage.Container)
	cfg.Storage.AzureAccountName = os.ExpandEnv(cfg.Storage.AzureAccountName)
	cfg.Storage.AzureAccountKey = os.ExpandEnv(cfg.Storage.AzureAccountKey)
	cfg.Storage.GCSCredentialsFile = os.ExpandEnv(cfg.Storage.GCSCredentialsFile)

	cfg.Restore.BackupPath = os.ExpandEnv(cfg.Restore.BackupPath)
	cfg.Logging.File = os.ExpandEnv(cfg.Logging.File)
	cfg.Notifications.SlackWebhook = os.ExpandEnv(cfg.Notifications.SlackWebhook)
}

// Validate validates that the configuration meets semantic requirements.
func (cfg *Config) Validate(command string) error {
	// DB validation
	dbType := strings.ToLower(cfg.DB.Type)
	if dbType == "" {
		return errors.New("database type (db.type) is required")
	}
	if dbType != "postgres" && dbType != "mysql" && dbType != "mongodb" && dbType != "sqlite" {
		return fmt.Errorf("unsupported database type: %s (must be postgres, mysql, mongodb, sqlite)", cfg.DB.Type)
	}

	if dbType == "sqlite" {
		if cfg.DB.Database == "" {
			return errors.New("database path (db.database) is required for sqlite")
		}
	} else {
		// Remote DBs
		if cfg.DB.Host == "" {
			return fmt.Errorf("database host (db.host) is required for %s", dbType)
		}
		if cfg.DB.Database == "" {
			return fmt.Errorf("database name (db.database) is required for %s", dbType)
		}
	}

	// Backup / Restore specific validation
	if command == "backup" {
		mode := strings.ToLower(cfg.Backup.Mode)
		if mode != "full" && mode != "incremental" && mode != "differential" {
			return fmt.Errorf("unsupported backup mode: %s (must be full, incremental, or differential)", cfg.Backup.Mode)
		}
		compress := strings.ToLower(cfg.Backup.Compress)
		if compress != "none" && compress != "gzip" && compress != "zstd" {
			return fmt.Errorf("unsupported compression format: %s (must be none, gzip, or zstd)", cfg.Backup.Compress)
		}

		storageType := strings.ToLower(cfg.Storage.Type)
		if storageType == "" {
			return errors.New("storage type (storage.type) is required for backup")
		}
		switch storageType {
		case "local":
			if cfg.Storage.LocalPath == "" {
				return errors.New("local path (storage.local_path) is required for local storage")
			}
		case "s3":
			if cfg.Storage.Bucket == "" {
				return errors.New("bucket (storage.bucket) is required for s3 storage")
			}
		case "gcs":
			if cfg.Storage.Bucket == "" {
				return errors.New("bucket (storage.bucket) is required for gcs storage")
			}
		case "azure":
			if cfg.Storage.Container == "" {
				return errors.New("container (storage.container) is required for azure storage")
			}
			if cfg.Storage.AzureAccountName == "" {
				return errors.New("azure account name (storage.azure_account_name) is required for azure storage")
			}
		default:
			return fmt.Errorf("unsupported storage type: %s (must be local, s3, gcs, azure)", cfg.Storage.Type)
		}
	} else if command == "restore" {
		if cfg.Restore.BackupPath == "" {
			return errors.New("backup path (restore.backup_path) is required for restore")
		}
	}

	return nil
}

// Redact returns a deep copy of the config with credentials hidden.
func (cfg *Config) Redact() *Config {
	redacted := *cfg

	if redacted.DB.Password != "" {
		redacted.DB.Password = "[REDACTED]"
	}
	if redacted.Storage.SecretKey != "" {
		redacted.Storage.SecretKey = "[REDACTED]"
	}
	if redacted.Storage.AzureAccountKey != "" {
		redacted.Storage.AzureAccountKey = "[REDACTED]"
	}

	return &redacted
}
