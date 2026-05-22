package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		cmd     string
		wantErr bool
	}{
		{
			name: "Valid SQLite Backup",
			config: Config{
				DB: DBConfig{
					Type:     "sqlite",
					Database: "mydb.sqlite",
				},
				Backup: BackupConfig{
					Mode:     "full",
					Compress: "gzip",
				},
				Storage: StorageConfig{
					Type:      "local",
					LocalPath: "/tmp",
				},
			},
			cmd:     "backup",
			wantErr: false,
		},
		{
			name: "Valid Postgres Backup to S3",
			config: Config{
				DB: DBConfig{
					Type:     "postgres",
					Host:     "localhost",
					Database: "appdb",
				},
				Backup: BackupConfig{
					Mode:     "full",
					Compress: "zstd",
				},
				Storage: StorageConfig{
					Type:   "s3",
					Bucket: "mybucket",
				},
			},
			cmd:     "backup",
			wantErr: false,
		},
		{
			name: "Invalid SQLite (Missing database path)",
			config: Config{
				DB: DBConfig{
					Type: "sqlite",
				},
				Backup: BackupConfig{
					Mode:     "full",
					Compress: "gzip",
				},
				Storage: StorageConfig{
					Type:      "local",
					LocalPath: "/tmp",
				},
			},
			cmd:     "backup",
			wantErr: true,
		},
		{
			name: "Invalid Postgres (Missing host)",
			config: Config{
				DB: DBConfig{
					Type:     "postgres",
					Database: "appdb",
				},
				Backup: BackupConfig{
					Mode:     "full",
					Compress: "gzip",
				},
				Storage: StorageConfig{
					Type:      "local",
					LocalPath: "/tmp",
				},
			},
			cmd:     "backup",
			wantErr: true,
		},
		{
			name: "Invalid storage type",
			config: Config{
				DB: DBConfig{
					Type:     "postgres",
					Host:     "localhost",
					Database: "appdb",
				},
				Backup: BackupConfig{
					Mode:     "full",
					Compress: "gzip",
				},
				Storage: StorageConfig{
					Type: "invalid",
				},
			},
			cmd:     "backup",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate(tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigRedact(t *testing.T) {
	cfg := Config{
		DB: DBConfig{
			Type:     "postgres",
			Password: "supersecretpassword",
		},
		Storage: StorageConfig{
			Type:            "s3",
			SecretKey:       "awssecret",
			AzureAccountKey: "azuresecret",
		},
	}

	redacted := cfg.Redact()

	if redacted.DB.Password != "[REDACTED]" {
		t.Errorf("Expected redacted DB password to be [REDACTED], got %s", redacted.DB.Password)
	}
	if redacted.Storage.SecretKey != "[REDACTED]" {
		t.Errorf("Expected redacted Storage secret key to be [REDACTED], got %s", redacted.Storage.SecretKey)
	}
	if redacted.Storage.AzureAccountKey != "[REDACTED]" {
		t.Errorf("Expected redacted Storage azure key to be [REDACTED], got %s", redacted.Storage.AzureAccountKey)
	}

	// Verify original config was not modified
	if cfg.DB.Password != "supersecretpassword" {
		t.Errorf("Original DB password was mutated: %s", cfg.DB.Password)
	}
}

func TestLoadConfigEnvAndDefaults(t *testing.T) {
	os.Setenv("BACKUPDB_DB_TYPE", "mysql")
	os.Setenv("BACKUPDB_DB_HOST", "myhost")
	os.Setenv("BACKUPDB_DB_PASSWORD", "secret")
	defer func() {
		os.Unsetenv("BACKUPDB_DB_TYPE")
		os.Unsetenv("BACKUPDB_DB_HOST")
		os.Unsetenv("BACKUPDB_DB_PASSWORD")
	}()

	cfg, err := LoadConfig(viper.New(), "")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.DB.Type != "mysql" {
		t.Errorf("Expected DB.Type to be mysql, got %s", cfg.DB.Type)
	}
	if cfg.DB.Host != "myhost" {
		t.Errorf("Expected DB.Host to be myhost, got %s", cfg.DB.Host)
	}
	if cfg.DB.Password != "secret" {
		t.Errorf("Expected DB.Password to be secret, got %s", cfg.DB.Password)
	}

	// Verify default values
	if cfg.Backup.Mode != "full" {
		t.Errorf("Expected default backup mode to be full, got %s", cfg.Backup.Mode)
	}
	if cfg.Backup.Compress != "gzip" {
		t.Errorf("Expected default compression to be gzip, got %s", cfg.Backup.Compress)
	}
}
