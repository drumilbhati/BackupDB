package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/drumilbhati/BackupDB/internal/config"
	_ "github.com/go-sql-driver/mysql" // just to satisfy drivers loading if any side effects
)

func TestOrchestratorSQLiteE2E(t *testing.T) {
	// Check if sqlite3 CLI is available
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 CLI not found in PATH, skipping integration test")
	}

	tempDir, err := os.MkdirTemp("", "backupdb_orch_test_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	restoredDbPath := filepath.Join(tempDir, "test_restored.db")
	storagePath := filepath.Join(tempDir, "backups")

	// 1. Create a dummy sqlite database and populate it
	cmd := exec.Command("sqlite3", dbPath, "CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT); INSERT INTO users(name) VALUES('Alice'); INSERT INTO users(name) VALUES('Bob');")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create source sqlite db: %v", err)
	}

	// 2. Define Backup config
	cfg := &config.Config{
		DB: config.DBConfig{
			Type:     "sqlite",
			Database: dbPath,
		},
		Backup: config.BackupConfig{
			Mode:     "full",
			Compress: "gzip",
		},
		Storage: config.StorageConfig{
			Type:      "local",
			LocalPath: storagePath,
		},
		Catalog: config.CatalogConfig{
			Path: filepath.Join(tempDir, "catalog.json"),
		},
		Output: config.OutputConfig{
			Format: "text",
		},
	}

	// 3. Run Backup
	orch := NewOrchestrator(cfg)
	outcome, err, code := orch.RunBackup()
	if err != nil {
		t.Fatalf("Backup failed with code %d: %v", code, err)
	}

	if outcome.Status != "success" {
		t.Errorf("Expected outcome status to be success, got %s", outcome.Status)
	}
	if outcome.Bytes <= 0 {
		t.Errorf("Expected written bytes to be > 0, got %d", outcome.Bytes)
	}
	if outcome.Checksum == "" {
		t.Error("Expected non-empty SHA256 checksum")
	}

	// Verify backup file exists
	if _, err := os.Stat(outcome.ArtifactURI); os.IsNotExist(err) {
		t.Fatalf("Backup file was not created: %s", outcome.ArtifactURI)
	}

	// 4. Define Restore config
	restoreCfg := &config.Config{
		DB: config.DBConfig{
			Type:     "sqlite",
			Database: restoredDbPath,
		},
		Restore: config.RestoreConfig{
			BackupPath: outcome.ArtifactURI,
		},
		Storage: config.StorageConfig{
			Type: "local",
		},
		Catalog: config.CatalogConfig{
			Path: filepath.Join(tempDir, "catalog.json"),
		},
		Output: config.OutputConfig{
			Format: "text",
		},
	}

	// 5. Run Restore
	orchRestore := NewOrchestrator(restoreCfg)
	restoreOutcome, err, code := orchRestore.RunRestore()
	if err != nil {
		t.Fatalf("Restore failed with code %d: %v", code, err)
	}

	if restoreOutcome.Status != "success" {
		t.Errorf("Expected restore outcome status to be success, got %s", restoreOutcome.Status)
	}

	// Verify restored db has correct data
	// Since sqlite is local, we check the database contents via sqlite3 shell query or file check
	verifyCmd := exec.Command("sqlite3", restoredDbPath, "SELECT name FROM users ORDER BY id;")
	outBytes, err := verifyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to query restored database: %v. Output: %s", err, string(outBytes))
	}

	expectedOutput := "Alice\nBob\n"
	if string(outBytes) != expectedOutput {
		t.Errorf("Restored data mismatch.\nExpected:\n%s\nGot:\n%s", expectedOutput, string(outBytes))
	}
}

func TestOrchestratorSQLiteIncrementalAndDifferential(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 CLI not found in PATH, skipping integration test")
	}

	tempDir, err := os.MkdirTemp("", "backupdb_chain_test_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "source.db")
	storagePath := filepath.Join(tempDir, "backups")
	catalogPath := filepath.Join(tempDir, "catalog.json")

	runSQL := func(dbPath string, sql string) {
		cmd := exec.Command("sqlite3", dbPath, sql)
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to run sqlite command: %v", err)
		}
	}

	runSQL(dbPath, "CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT); INSERT INTO users(name) VALUES('Alice'); INSERT INTO users(name) VALUES('Bob');")

	baseCfg := func(mode string) *config.Config {
		return &config.Config{
			DB: config.DBConfig{
				Type:     "sqlite",
				Database: dbPath,
			},
			Backup: config.BackupConfig{
				Mode:     mode,
				Compress: "gzip",
			},
			Storage: config.StorageConfig{
				Type:      "local",
				LocalPath: storagePath,
			},
			Catalog: config.CatalogConfig{
				Path: catalogPath,
			},
			Output: config.OutputConfig{
				Format: "text",
			},
		}
	}

	fullOut, err, code := NewOrchestrator(baseCfg("full")).RunBackup()
	if err != nil {
		t.Fatalf("full backup failed with code %d: %v", code, err)
	}

	runSQL(dbPath, "INSERT INTO users(name) VALUES('Charlie');")
	incOut, err, code := NewOrchestrator(baseCfg("incremental")).RunBackup()
	if err != nil {
		t.Fatalf("incremental backup failed with code %d: %v", code, err)
	}

	runSQL(dbPath, "INSERT INTO users(name) VALUES('Diana');")
	diffOut, err, code := NewOrchestrator(baseCfg("differential")).RunBackup()
	if err != nil {
		t.Fatalf("differential backup failed with code %d: %v", code, err)
	}

	restoreExpected := func(backupPath, restoredDB, expected string) {
		cfg := &config.Config{
			DB: config.DBConfig{
				Type:     "sqlite",
				Database: restoredDB,
			},
			Restore: config.RestoreConfig{
				BackupPath: backupPath,
			},
			Storage: config.StorageConfig{
				Type: "local",
			},
			Catalog: config.CatalogConfig{
				Path: catalogPath,
			},
			Output: config.OutputConfig{
				Format: "text",
			},
		}

		outcome, err, code := NewOrchestrator(cfg).RunRestore()
		if err != nil {
			t.Fatalf("restore failed with code %d: %v", code, err)
		}
		if outcome.Status != "success" {
			t.Fatalf("expected restore status success, got %s", outcome.Status)
		}

		verifyCmd := exec.Command("sqlite3", restoredDB, "SELECT name FROM users ORDER BY id;")
		outBytes, err := verifyCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("failed to query restored database: %v. Output: %s", err, string(outBytes))
		}
		if string(outBytes) != expected {
			t.Fatalf("restored data mismatch.\nExpected:\n%s\nGot:\n%s", expected, string(outBytes))
		}
	}

	incDB := filepath.Join(tempDir, "restored_inc.db")
	diffDB := filepath.Join(tempDir, "restored_diff.db")
	restoreExpected(incOut.ArtifactURI, incDB, "Alice\nBob\nCharlie\n")
	restoreExpected(diffOut.ArtifactURI, diffDB, "Alice\nBob\nCharlie\nDiana\n")

	if fullOut.BackupID == "" || incOut.BackupID == "" || diffOut.BackupID == "" {
		t.Fatal("expected backup IDs for chain backups")
	}
}

// TestValidateSuccess tests connection validation logic with a valid local file
func TestOrchestratorValidate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "backupdb_val_test_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	f, _ := os.Create(dbPath)
	f.Close()

	cfg := &config.Config{
		DB: config.DBConfig{
			Type:     "sqlite",
			Database: dbPath,
		},
	}

	orch := NewOrchestrator(cfg)
	err, code := orch.RunValidate()
	if err != nil {
		t.Errorf("Expected RunValidate to succeed, got error: %v, code: %d", err, code)
	}

	// Test invalid path validation
	cfgInvalid := &config.Config{
		DB: config.DBConfig{
			Type:     "sqlite",
			Database: filepath.Join(tempDir, "non_existent_dir", "db.sqlite"),
		},
	}
	orchInvalid := NewOrchestrator(cfgInvalid)
	err, code = orchInvalid.RunValidate()
	if err == nil {
		t.Error("Expected RunValidate to fail for invalid database directory path")
	}
	if code != ExitConnectionFailure {
		t.Errorf("Expected exit code ExitConnectionFailure (3), got %d", code)
	}
}
