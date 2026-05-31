package db

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"time"

	"github.com/drumilbhati/BackupDB/internal/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoDBHandler struct{}

func NewMongoDBHandler() DbHandler {
	return &MongoDBHandler{}
}

func (h *MongoDBHandler) ValidateConnection(conn config.DBConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	port := conn.Port
	if port == 0 {
		port = 27017
	}

	var uri string
	if conn.User != "" && conn.Password != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d", conn.User, conn.Password, conn.Host, port)
	} else {
		uri = fmt.Sprintf("mongodb://%s:%d", conn.Host, port)
	}

	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return fmt.Errorf("failed to configure mongodb client: %w", err)
	}
	defer func() {
		_ = client.Disconnect(ctx)
	}()

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return fmt.Errorf("failed to ping mongodb: %w", err)
	}

	return nil
}

func (h *MongoDBHandler) PrepareBackup(cfg *config.Config) (*BackupContext, error) {
	port := cfg.DB.Port
	if port == 0 {
		port = 27017
	}

	args := []string{
		"--host", cfg.DB.Host,
		"--port", strconv.Itoa(port),
		"--db", cfg.DB.Database,
		"--archive", // Streams archive directly to stdout
	}

	if cfg.DB.User != "" {
		args = append(args, "--username", cfg.DB.User)
	}
	if cfg.DB.Password != "" {
		args = append(args, "--password", cfg.DB.Password)
	}

	return &BackupContext{
		DBConfig: cfg.DB,
		DBName:   cfg.DB.Database,
		CmdArgs:  args,
	}, nil
}

func (h *MongoDBHandler) StreamBackup(ctx *BackupContext, sink io.Writer) (*BackupStats, error) {
	cmd := exec.Command("mongodump", ctx.CmdArgs...)
	cmd.Stdout = sink

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mongodump: %w", err)
	}

	stderrBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("mongodump failed: %s (%w)", string(stderrBytes), err)
	}

	return &BackupStats{}, nil
}

func (h *MongoDBHandler) FinalizeBackup(ctx *BackupContext, stats *BackupStats) error {
	return nil
}

func (h *MongoDBHandler) PrepareRestore(cfg *config.Config) (*RestoreContext, error) {
	port := cfg.DB.Port
	if port == 0 {
		port = 27017
	}

	args := []string{
		"--host", cfg.DB.Host,
		"--port", strconv.Itoa(port),
		"--archive", // Reads archive from stdin
	}

	if cfg.DB.User != "" {
		args = append(args, "--username", cfg.DB.User)
	}
	if cfg.DB.Password != "" {
		args = append(args, "--password", cfg.DB.Password)
	}

	return &RestoreContext{
		DBConfig: cfg.DB,
		DBName:   cfg.DB.Database,
		CmdArgs:  args,
	}, nil
}

func (h *MongoDBHandler) StreamRestore(ctx *RestoreContext, source io.Reader) (*RestoreStats, error) {
	cmd := exec.Command("mongorestore", ctx.CmdArgs...)
	cmd.Stdin = source

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mongorestore: %w", err)
	}

	stderrBytes, _ := io.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("mongorestore failed: %s (%w)", string(stderrBytes), err)
	}

	return &RestoreStats{}, nil
}

func (h *MongoDBHandler) FinalizeRestore(ctx *RestoreContext, stats *RestoreStats) error {
	return nil
}

func (h *MongoDBHandler) SupportsMode(mode string) bool {
	switch mode {
	case "full", "incremental", "differential":
		return true
	default:
		return false
	}
}

func (h *MongoDBHandler) SupportsSelectiveRestore() bool {
	return false
}
