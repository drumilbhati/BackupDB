package orchestrator

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/drumilbhati/BackupDB/internal/config"
	"github.com/drumilbhati/BackupDB/internal/db"
	"github.com/drumilbhati/BackupDB/internal/storage"
	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
)

// Exit codes matching root.go
const (
	ExitSuccess           = 0
	ExitInvalidInput      = 2
	ExitConnectionFailure = 3
	ExitBackupFailure     = 4
	ExitRestoreFailure    = 5
	ExitStorageFailure    = 6
)

type Orchestrator struct {
	cfg *config.Config
}

type BackupOutcome struct {
	BackupID    string `json:"backup_id"`
	ArtifactURI string `json:"artifact_uri"`
	Bytes       int64  `json:"bytes"`
	Checksum    string `json:"checksum"`
	DurationMs  int64  `json:"duration_ms"`
	Status      string `json:"status"`
}

type RestoreOutcome struct {
	RestoredObjects int64  `json:"restored_objects"`
	DurationMs      int64  `json:"duration_ms"`
	Status          string `json:"status"`
}

func NewOrchestrator(cfg *config.Config) *Orchestrator {
	return &Orchestrator{cfg: cfg}
}

func (o *Orchestrator) SelectDbHandler(dbType string) (db.DbHandler, error) {
	switch strings.ToLower(dbType) {
	case "postgres":
		return db.NewPostgresHandler(), nil
	case "mysql":
		return db.NewMySQLHandler(), nil
	case "mongodb":
		return db.NewMongoDBHandler(), nil
	case "sqlite":
		return db.NewSQLiteHandler(), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

func (o *Orchestrator) SelectStorageAdapter(storageType string) (storage.StorageAdapter, error) {
	switch strings.ToLower(storageType) {
	case "local":
		return storage.NewLocalAdapter(o.cfg.Storage), nil
	case "s3":
		return storage.NewS3Adapter(o.cfg.Storage), nil
	case "gcs":
		return storage.NewGCSAdapter(o.cfg.Storage), nil
	case "azure":
		return storage.NewAzureAdapter(o.cfg.Storage), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

func (o *Orchestrator) RunValidate() (error, int) {
	dbHandler, err := o.SelectDbHandler(o.cfg.DB.Type)
	if err != nil {
		return err, ExitInvalidInput
	}

	err = dbHandler.ValidateConnection(o.cfg.DB)
	if err != nil {
		return err, ExitConnectionFailure
	}

	return nil, ExitSuccess
}

func (o *Orchestrator) RunBackup() (*BackupOutcome, error, int) {
	startTime := time.Now()
	backupID := uuid.New().String()

	dbHandler, err := o.SelectDbHandler(o.cfg.DB.Type)
	if err != nil {
		return nil, err, ExitInvalidInput
	}

	storageAdapter, err := o.SelectStorageAdapter(o.cfg.Storage.Type)
	if err != nil {
		return nil, err, ExitInvalidInput
	}

	// 1. Connection check
	if err := dbHandler.ValidateConnection(o.cfg.DB); err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, fmt.Errorf("database connection failed: %w", err), ExitConnectionFailure
	}

	// 2. Storage target check
	if err := storageAdapter.ValidateTarget(); err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, fmt.Errorf("storage target validation failed: %w", err), ExitStorageFailure
	}

	// 3. Check mode support
	if !dbHandler.SupportsMode(o.cfg.Backup.Mode) {
		err := fmt.Errorf("database handler %s does not support backup mode: %s", o.cfg.DB.Type, o.cfg.Backup.Mode)
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, err, ExitInvalidInput
	}

	backupCtx, err := dbHandler.PrepareBackup(o.cfg)
	if err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, fmt.Errorf("failed to prepare backup: %w", err), ExitBackupFailure
	}

	pr, pw := io.Pipe()

	// Wrap pipe writer with compressor
	var compWriter io.WriteCloser
	var compErr error
	compressFormat := strings.ToLower(o.cfg.Backup.Compress)
	if compressFormat == "gzip" {
		compWriter = gzip.NewWriter(pw)
	} else if compressFormat == "zstd" {
		compWriter, compErr = zstd.NewWriter(pw)
		if compErr != nil {
			pw.CloseWithError(compErr)
			o.notifySlack(backupID, "failure", 0, 0, "", compErr)
			return nil, fmt.Errorf("failed to initialize zstd compressor: %w", compErr), ExitBackupFailure
		}
	} else {
		compWriter = &nopWriteCloser{pw}
	}

	// Streaming backup in a goroutine
	go func() {
		var streamErr error
		defer func() {
			compWriter.Close()
			if streamErr != nil {
				pw.CloseWithError(streamErr)
			} else {
				pw.Close()
			}
		}()

		stats, err := dbHandler.StreamBackup(backupCtx, compWriter)
		if err != nil {
			streamErr = err
		}
		_ = stats
	}()

	// Read from pipe, calculate SHA256 and size, and write to storage
	hasher := sha256.New()
	tr := &trackingReader{
		r: pr,
		h: hasher,
	}

	meta := storage.ArtifactMetadata{
		DBType:      o.cfg.DB.Type,
		BackupMode:  o.cfg.Backup.Mode,
		Timestamp:   time.Now().Format("20060102_150405"),
		Compression: compressFormat,
	}

	ref, err := storageAdapter.Write(tr, meta)
	if err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, fmt.Errorf("storage write failed: %w", err), ExitStorageFailure
	}

	// Post-write metrics update
	checksumStr := hex.EncodeToString(hasher.Sum(nil))
	ref.Checksum = checksumStr
	ref.SizeBytes = tr.n

	if err := dbHandler.FinalizeBackup(backupCtx, &db.BackupStats{BytesWritten: tr.n}); err != nil {
		o.notifySlack(backupID, "failure", tr.n, 0, ref.URI, err)
		return nil, fmt.Errorf("failed to finalize backup: %w", err), ExitBackupFailure
	}

	duration := time.Since(startTime)
	outcome := &BackupOutcome{
		BackupID:    backupID,
		ArtifactURI: ref.URI,
		Bytes:       tr.n,
		Checksum:    checksumStr,
		DurationMs:  duration.Milliseconds(),
		Status:      "success",
	}

	o.notifySlack(backupID, "success", tr.n, duration, ref.URI, nil)

	return outcome, nil, ExitSuccess
}

func (o *Orchestrator) RunRestore() (*RestoreOutcome, error, int) {
	startTime := time.Now()

	dbHandler, err := o.SelectDbHandler(o.cfg.DB.Type)
	if err != nil {
		return nil, err, ExitInvalidInput
	}

	storageAdapter, err := o.SelectStorageAdapter(o.cfg.Storage.Type)
	if err != nil {
		return nil, err, ExitInvalidInput
	}

	// 1. Connection check
	if err := dbHandler.ValidateConnection(o.cfg.DB); err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err), ExitConnectionFailure
	}

	restoreCtx, err := dbHandler.PrepareRestore(o.cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare restore: %w", err), ExitRestoreFailure
	}

	// Retrieve backup stream
	ref := storage.ArtifactRef{
		URI:         o.cfg.Restore.BackupPath,
		StorageType: o.cfg.Storage.Type,
	}

	rc, err := storageAdapter.Read(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup from storage: %w", err), ExitStorageFailure
	}
	defer rc.Close()

	// Decompress stream based on path extension
	var decompressed io.Reader
	lowerPath := strings.ToLower(o.cfg.Restore.BackupPath)
	if strings.HasSuffix(lowerPath, ".gz") {
		gr, err := gzip.NewReader(rc)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize gzip decompressor: %w", err), ExitRestoreFailure
		}
		defer gr.Close()
		decompressed = gr
	} else if strings.HasSuffix(lowerPath, ".zst") {
		zr, err := zstd.NewReader(rc)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize zstd decompressor: %w", err), ExitRestoreFailure
		}
		defer zr.Close()
		decompressed = zr
	} else {
		decompressed = rc
	}

	stats, err := dbHandler.StreamRestore(restoreCtx, decompressed)
	if err != nil {
		return nil, fmt.Errorf("restore stream failed: %w", err), ExitRestoreFailure
	}

	if err := dbHandler.FinalizeRestore(restoreCtx, stats); err != nil {
		return nil, fmt.Errorf("failed to finalize restore: %w", err), ExitRestoreFailure
	}

	duration := time.Since(startTime)
	outcome := &RestoreOutcome{
		RestoredObjects: stats.ObjectsRestored,
		DurationMs:      duration.Milliseconds(),
		Status:          "success",
	}

	return outcome, nil, ExitSuccess
}

func (o *Orchestrator) notifySlack(backupID, status string, sizeBytes int64, duration time.Duration, uri string, err error) {
	webhook := o.cfg.Notifications.SlackWebhook
	if webhook == "" {
		return
	}

	var statusEmoji string
	var color string
	if status == "success" {
		statusEmoji = ":white_check_mark:"
		color = "#36a64f"
	} else {
		statusEmoji = ":warning:"
		color = "#ff0000"
	}

	title := fmt.Sprintf("%s BackupDB Notification: %s", statusEmoji, strings.ToUpper(status))

	sizeMB := float64(sizeBytes) / (1024 * 1024)

	var errText string
	if err != nil {
		errText = fmt.Sprintf("\n*Error:* %s", err.Error())
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"fallback": title,
				"color":    color,
				"pretext":  "Database Backup Job execution finished",
				"title":    title,
				"text": fmt.Sprintf(
					"*Backup ID:* `%s`"+
						"\n*Database Type:* `%s`"+
						"\n*Database Name:* `%s`"+
						"\n*Size:* `%.2f MB`"+
						"\n*Duration:* `%s`"+
						"\n*Target URI:* `%s`"+
						"%s",
					backupID, o.cfg.DB.Type, o.cfg.DB.Database, sizeMB, duration.String(), uri, errText,
				),
				"ts": time.Now().Unix(),
			},
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", webhook, bytes.NewReader(jsonBytes))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

type trackingReader struct {
	r io.Reader
	h hash.Hash
	n int64
}

func (tr *trackingReader) Read(p []byte) (int, error) {
	n, err := tr.r.Read(p)
	if n > 0 {
		tr.h.Write(p[:n])
		tr.n += int64(n)
	}
	return n, err
}

type nopWriteCloser struct {
	w io.Writer
}

func (n *nopWriteCloser) Write(p []byte) (int, error) {
	return n.w.Write(p)
}

func (n *nopWriteCloser) Close() error {
	return nil
}
