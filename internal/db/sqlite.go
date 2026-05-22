package db

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/drumilbhati/BackupDB/internal/config"
)

type SQLiteHandler struct{}

func NewSQLiteHandler() DbHandler {
	return &SQLiteHandler{}
}

func (h *SQLiteHandler) ValidateConnection(conn config.DBConfig) error {
	if conn.Database == "" {
		return fmt.Errorf("sqlite database path cannot be empty")
	}

	// Resolve the absolute path
	absPath, err := filepath.Abs(conn.Database)
	if err != nil {
		return fmt.Errorf("failed to resolve path %s: %w", conn.Database, err)
	}

	// Check if file exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		// If it doesn't exist, check if we can create it (check if directory is writeable)
		dir := filepath.Dir(absPath)
		dirInfo, dirErr := os.Stat(dir)
		if os.IsNotExist(dirErr) {
			return fmt.Errorf("directory for sqlite database does not exist: %s", dir)
		}
		if !dirInfo.IsDir() {
			return fmt.Errorf("path is not a directory: %s", dir)
		}
		// Try creating a temporary file in that directory to check write permissions
		tempFile, tempErr := os.CreateTemp(dir, "backupdb_test_")
		if tempErr != nil {
			return fmt.Errorf("directory %s is not writable: %w", dir, tempErr)
		}
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to access sqlite database: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("sqlite database path is a directory: %s", absPath)
	}

	// Check if read/write permissions are granted
	file, err := os.OpenFile(absPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("sqlite database file is not read/writeable: %w", err)
	}
	file.Close()

	return nil
}

func (h *SQLiteHandler) PrepareBackup(cfg *config.Config) (*BackupContext, error) {
	// sqlite3 db_path .dump
	args := []string{
		cfg.DB.Database,
		".dump",
	}

	return &BackupContext{
		DBConfig: cfg.DB,
		DBName:   cfg.DB.Database,
		CmdArgs:  args,
	}, nil
}

func (h *SQLiteHandler) StreamBackup(ctx *BackupContext, sink io.Writer) (*BackupStats, error) {
	// Verify sqlite3 is available in PATH
	if _, err := exec.LookPath("sqlite3"); err != nil {
		return nil, fmt.Errorf("sqlite3 CLI tool not found in PATH: %w", err)
	}

	cmd := exec.Command("sqlite3", ctx.CmdArgs...)
	cmd.Stdout = sink
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start sqlite3: %w", err)
	}

	stderrBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("sqlite3 backup failed: %s (%w)", string(stderrBytes), err)
	}

	return &BackupStats{}, nil
}

func (h *SQLiteHandler) FinalizeBackup(ctx *BackupContext, stats *BackupStats) error {
	return nil
}

func (h *SQLiteHandler) PrepareRestore(cfg *config.Config) (*RestoreContext, error) {
	// sqlite3 db_path
	args := []string{
		cfg.DB.Database,
	}

	return &RestoreContext{
		DBConfig: cfg.DB,
		DBName:   cfg.DB.Database,
		CmdArgs:  args,
	}, nil
}

func (h *SQLiteHandler) StreamRestore(ctx *RestoreContext, source io.Reader) (*RestoreStats, error) {
	// Verify sqlite3 is available in PATH
	if _, err := exec.LookPath("sqlite3"); err != nil {
		return nil, fmt.Errorf("sqlite3 CLI tool not found in PATH: %w", err)
	}

	// Remove target file first to ensure a clean restore
	_ = os.Remove(ctx.DBConfig.Database)

	cmd := exec.Command("sqlite3", ctx.CmdArgs...)
	cmd.Stdin = source
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start sqlite3 restore: %w", err)
	}

	stderrBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("sqlite3 restore failed: %s (%w)", string(stderrBytes), err)
	}

	return &RestoreStats{}, nil
}

func (h *SQLiteHandler) FinalizeRestore(ctx *RestoreContext, stats *RestoreStats) error {
	return nil
}

func (h *SQLiteHandler) SupportsMode(mode string) bool {
	return mode == "full"
}

func (h *SQLiteHandler) SupportsSelectiveRestore() bool {
	return false
}
