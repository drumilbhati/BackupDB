package db

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"time"

	"github.com/drumilbhati/BackupDB/internal/config"
	"github.com/jackc/pgx/v5"
)

type PostgresHandler struct{}

func NewPostgresHandler() DbHandler {
	return &PostgresHandler{}
}

func (h *PostgresHandler) ValidateConnection(conn config.DBConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	port := conn.Port
	if port == 0 {
		port = 5432
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		conn.User, conn.Password, conn.Host, port, conn.Database)

	cfg, err := pgx.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("invalid postgres connection string: %w", err)
	}

	connObj, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer connObj.Close(context.Background())

	err = connObj.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping postgres: %w", err)
	}

	return nil
}

func (h *PostgresHandler) PrepareBackup(cfg *config.Config) (*BackupContext, error) {
	port := cfg.DB.Port
	if port == 0 {
		port = 5432
	}

	args := []string{
		"-h", cfg.DB.Host,
		"-p", strconv.Itoa(port),
		"-U", cfg.DB.User,
		"-d", cfg.DB.Database,
	}

	return &BackupContext{
		DBConfig: cfg.DB,
		DBName:   cfg.DB.Database,
		CmdArgs:  args,
	}, nil
}

func (h *PostgresHandler) StreamBackup(ctx *BackupContext, sink io.Writer) (*BackupStats, error) {
	cmd := exec.Command("pg_dump", ctx.CmdArgs...)
	
	// Set password env var
	if ctx.DBConfig.Password != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+ctx.DBConfig.Password)
	}

	cmd.Stdout = sink
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start pg_dump: %w", err)
	}

	stderrBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("pg_dump failed: %s (%w)", string(stderrBytes), err)
	}

	return &BackupStats{}, nil
}

func (h *PostgresHandler) FinalizeBackup(ctx *BackupContext, stats *BackupStats) error {
	return nil
}

func (h *PostgresHandler) PrepareRestore(cfg *config.Config) (*RestoreContext, error) {
	port := cfg.DB.Port
	if port == 0 {
		port = 5432
	}

	args := []string{
		"-h", cfg.DB.Host,
		"-p", strconv.Itoa(port),
		"-U", cfg.DB.User,
		"-d", cfg.DB.Database,
	}

	return &RestoreContext{
		DBConfig: cfg.DB,
		DBName:   cfg.DB.Database,
		CmdArgs:  args,
	}, nil
}

func (h *PostgresHandler) StreamRestore(ctx *RestoreContext, source io.Reader) (*RestoreStats, error) {
	// We use psql to restore plain SQL format backups
	cmd := exec.Command("psql", ctx.CmdArgs...)
	
	if ctx.DBConfig.Password != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+ctx.DBConfig.Password)
	}

	cmd.Stdin = source
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start psql: %w", err)
	}

	stderrBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("psql restore failed: %s (%w)", string(stderrBytes), err)
	}

	return &RestoreStats{}, nil
}

func (h *PostgresHandler) FinalizeRestore(ctx *RestoreContext, stats *RestoreStats) error {
	return nil
}

func (h *PostgresHandler) SupportsMode(mode string) bool {
	// Only full backup is supported via standard pg_dump
	return mode == "full"
}

func (h *PostgresHandler) SupportsSelectiveRestore() bool {
	return false
}
