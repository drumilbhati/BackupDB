package db

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/drumilbhati/BackupDB/internal/config"
	_ "github.com/go-sql-driver/mysql"
)

type MySQLHandler struct{}

func NewMySQLHandler() DbHandler {
	return &MySQLHandler{}
}

func (h *MySQLHandler) ValidateConnection(conn config.DBConfig) error {
	port := conn.Port
	if port == 0 {
		port = 3306
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=5s",
		conn.User, conn.Password, conn.Host, port, conn.Database)

	dbObj, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open mysql connection: %w", err)
	}
	defer dbObj.Close()

	dbObj.SetConnMaxLifetime(5 * time.Second)

	err = dbObj.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping mysql: %w", err)
	}

	return nil
}

func (h *MySQLHandler) PrepareBackup(cfg *config.Config) (*BackupContext, error) {
	port := cfg.DB.Port
	if port == 0 {
		port = 3306
	}

	args := []string{
		"-h", cfg.DB.Host,
		"-P", strconv.Itoa(port),
		"-u", cfg.DB.User,
		cfg.DB.Database,
	}

	return &BackupContext{
		DBConfig: cfg.DB,
		DBName:   cfg.DB.Database,
		CmdArgs:  args,
	}, nil
}

func (h *MySQLHandler) StreamBackup(ctx *BackupContext, sink io.Writer) (*BackupStats, error) {
	cmd := exec.Command("mysqldump", ctx.CmdArgs...)

	cmd.Env = os.Environ()
	if ctx.DBConfig.Password != "" {
		cmd.Env = append(cmd.Env, "MYSQL_PWD="+ctx.DBConfig.Password)
	}

	cmd.Stdout = sink

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mysqldump: %w", err)
	}

	stderrBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("mysqldump failed: %s (%w)", string(stderrBytes), err)
	}

	return &BackupStats{}, nil
}

func (h *MySQLHandler) FinalizeBackup(ctx *BackupContext, stats *BackupStats) error {
	return nil
}

func (h *MySQLHandler) PrepareRestore(cfg *config.Config) (*RestoreContext, error) {
	port := cfg.DB.Port
	if port == 0 {
		port = 3306
	}

	args := []string{
		"-h", cfg.DB.Host,
		"-P", strconv.Itoa(port),
		"-u", cfg.DB.User,
		cfg.DB.Database,
	}

	meta := make(map[string]interface{})
	if len(cfg.Restore.Tables) > 0 {
		meta["tables"] = cfg.Restore.Tables
	}

	return &RestoreContext{
		DBConfig: cfg.DB,
		DBName:   cfg.DB.Database,
		CmdArgs:  args,
		Metadata: meta,
	}, nil
}

func (h *MySQLHandler) StreamRestore(ctx *RestoreContext, source io.Reader) (*RestoreStats, error) {
	// Apply selective restore filter if tables are specified in metadata
	if tables, ok := ctx.Metadata["tables"].([]string); ok && len(tables) > 0 {
		source = NewSQLFilterReader(source, tables, false)
	}

	cmd := exec.Command("mysql", ctx.CmdArgs...)

	cmd.Env = os.Environ()
	if ctx.DBConfig.Password != "" {
		cmd.Env = append(cmd.Env, "MYSQL_PWD="+ctx.DBConfig.Password)
	}

	cmd.Stdin = source

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mysql restore: %w", err)
	}

	stderrBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("mysql restore failed: %s (%w)", string(stderrBytes), err)
	}

	return &RestoreStats{}, nil
}

func (h *MySQLHandler) FinalizeRestore(ctx *RestoreContext, stats *RestoreStats) error {
	return nil
}

func (h *MySQLHandler) SupportsMode(mode string) bool {
	switch mode {
	case "full", "incremental", "differential":
		return true
	default:
		return false
	}
}

func (h *MySQLHandler) SupportsSelectiveRestore() bool {
	return true
}
