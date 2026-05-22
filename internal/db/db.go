package db

import (
	"io"

	"github.com/drumilbhati/BackupDB/internal/config"
)

type BackupContext struct {
	DBConfig config.DBConfig
	DBName   string
	TmpPath  string
	CmdArgs  []string
	Metadata map[string]interface{}
}

type BackupStats struct {
	BytesWritten int64
}

type RestoreContext struct {
	DBConfig config.DBConfig
	DBName   string
	TmpPath  string
	CmdArgs  []string
	Metadata map[string]interface{}
}

type RestoreStats struct {
	ObjectsRestored int64
}

type DbHandler interface {
	ValidateConnection(conn config.DBConfig) error
	PrepareBackup(cfg *config.Config) (*BackupContext, error)
	StreamBackup(ctx *BackupContext, sink io.Writer) (*BackupStats, error)
	FinalizeBackup(ctx *BackupContext, stats *BackupStats) error
	PrepareRestore(cfg *config.Config) (*RestoreContext, error)
	StreamRestore(ctx *RestoreContext, source io.Reader) (*RestoreStats, error)
	FinalizeRestore(ctx *RestoreContext, stats *RestoreStats) error
	SupportsMode(mode string) bool
	SupportsSelectiveRestore() bool
}
