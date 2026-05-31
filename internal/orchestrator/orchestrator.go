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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drumilbhati/BackupDB/internal/backupdelta"
	"github.com/drumilbhati/BackupDB/internal/catalog"
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

func (o *Orchestrator) catalogPath() string {
	if strings.TrimSpace(o.cfg.Catalog.Path) == "" {
		return "./.backupdb/catalog.json"
	}
	return o.cfg.Catalog.Path
}

func (o *Orchestrator) loadCatalog() (*catalog.Catalog, error) {
	return catalog.Load(o.catalogPath())
}

func (o *Orchestrator) saveCatalog(c *catalog.Catalog) error {
	return c.Save(o.catalogPath())
}

func (o *Orchestrator) selectBasisEntry(c *catalog.Catalog, mode string) (*catalog.Entry, error) {
	dbType := o.cfg.DB.Type
	dbName := o.cfg.DB.Database

	switch strings.ToLower(mode) {
	case "incremental":
		entry, ok := c.LatestForDatabase(dbType, dbName)
		if !ok {
			return nil, fmt.Errorf("no previous backup found for incremental mode; create a full backup first")
		}
		return entry, nil
	case "differential":
		entry, ok := c.LatestFullForDatabase(dbType, dbName)
		if !ok {
			return nil, fmt.Errorf("no full backup found for differential mode; create a full backup first")
		}
		return entry, nil
	default:
		return nil, nil
	}
}

func (o *Orchestrator) readArtifactBytes(adapter storage.StorageAdapter, uri string) ([]byte, error) {
	ref := storage.ArtifactRef{URI: uri, StorageType: o.cfg.Storage.Type}
	rc, err := adapter.Read(ref)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var reader io.Reader = rc
	lowerPath := strings.ToLower(uri)
	if strings.HasSuffix(lowerPath, ".gz") {
		gr, err := gzip.NewReader(rc)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize gzip decompressor: %w", err)
		}
		defer gr.Close()
		reader = gr
	} else if strings.HasSuffix(lowerPath, ".zst") {
		zr, err := zstd.NewReader(rc)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize zstd decompressor: %w", err)
		}
		defer zr.Close()
		reader = zr
	}

	return io.ReadAll(reader)
}

func (o *Orchestrator) resolveEntryBytes(c *catalog.Catalog, adapter storage.StorageAdapter, entry *catalog.Entry) ([]byte, error) {
	if entry == nil {
		return nil, fmt.Errorf("missing catalog entry")
	}

	chain, err := c.ChainTo(entry)
	if err != nil {
		return nil, err
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("empty backup chain")
	}

	current, err := o.readArtifactBytes(adapter, chain[0].ArtifactURI)
	if err != nil {
		return nil, err
	}

	for i := 1; i < len(chain); i++ {
		patchBytes, err := o.readArtifactBytes(adapter, chain[i].ArtifactURI)
		if err != nil {
			return nil, err
		}
		current, err = backupdelta.ApplyDelta(current, patchBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to apply backup delta for %s: %w", chain[i].BackupID, err)
		}
	}

	return current, nil
}

func compressArtifactBytes(raw []byte, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "gzip":
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(raw); err != nil {
			_ = gw.Close()
			return nil, fmt.Errorf("failed to compress artifact with gzip: %w", err)
		}
		if err := gw.Close(); err != nil {
			return nil, fmt.Errorf("failed to finalize gzip artifact: %w", err)
		}
		return buf.Bytes(), nil
	case "zstd":
		var buf bytes.Buffer
		zw, err := zstd.NewWriter(&buf)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize zstd compressor: %w", err)
		}
		if _, err := zw.Write(raw); err != nil {
			zw.Close()
			return nil, fmt.Errorf("failed to compress artifact with zstd: %w", err)
		}
		if err := zw.Close(); err != nil {
			return nil, fmt.Errorf("failed to finalize zstd artifact: %w", err)
		}
		return buf.Bytes(), nil
	default:
		return raw, nil
	}
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

	cat, err := o.loadCatalog()
	if err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, err, ExitBackupFailure
	}

	mode := strings.ToLower(o.cfg.Backup.Mode)
	basisEntry, err := o.selectBasisEntry(cat, mode)
	if err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, fmt.Errorf("failed to resolve backup basis: %w", err), ExitBackupFailure
	}

	tempDir, err := os.MkdirTemp("", "backupdb_dump_*")
	if err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, fmt.Errorf("failed to create temp backup directory: %w", err), ExitBackupFailure
	}
	defer os.RemoveAll(tempDir)

	dumpPath := filepath.Join(tempDir, "current.dump")
	dumpFile, err := os.Create(dumpPath)
	if err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, fmt.Errorf("failed to create temp dump file: %w", err), ExitBackupFailure
	}

	stats, streamErr := dbHandler.StreamBackup(backupCtx, dumpFile)
	closeErr := dumpFile.Close()
	if streamErr != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", streamErr)
		return nil, fmt.Errorf("backup stream failed: %w", streamErr), ExitBackupFailure
	}
	if closeErr != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", closeErr)
		return nil, fmt.Errorf("failed to close temp dump file: %w", closeErr), ExitBackupFailure
	}

	currentDumpBytes, err := os.ReadFile(dumpPath)
	if err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, fmt.Errorf("failed to read temp dump file: %w", err), ExitBackupFailure
	}

	if err := dbHandler.FinalizeBackup(backupCtx, stats); err != nil {
		o.notifySlack(backupID, "failure", int64(len(currentDumpBytes)), 0, "", err)
		return nil, fmt.Errorf("failed to finalize backup: %w", err), ExitBackupFailure
	}

	artifactBytes := currentDumpBytes
	artifactKind := "full"
	basisID := ""
	parentID := ""
	chainID := backupID
	sequence := 1

	switch mode {
	case "incremental":
		if basisEntry == nil {
			err := fmt.Errorf("incremental backup requires a previous backup")
			o.notifySlack(backupID, "failure", 0, 0, "", err)
			return nil, err, ExitInvalidInput
		}
		basisID = basisEntry.BackupID
		parentID = basisEntry.BackupID
		chainID = basisEntry.ChainID
		sequence = basisEntry.Sequence + 1
		baseBytes, err := o.resolveEntryBytes(cat, storageAdapter, basisEntry)
		if err != nil {
			o.notifySlack(backupID, "failure", 0, 0, "", err)
			return nil, fmt.Errorf("failed to resolve incremental basis: %w", err), ExitBackupFailure
		}
		artifactBytes, err = backupdelta.EncodeDelta(baseBytes, currentDumpBytes)
		if err != nil {
			o.notifySlack(backupID, "failure", 0, 0, "", err)
			return nil, fmt.Errorf("failed to create incremental delta: %w", err), ExitBackupFailure
		}
		artifactKind = "patch"
	case "differential":
		if basisEntry == nil {
			err := fmt.Errorf("differential backup requires a previous full backup")
			o.notifySlack(backupID, "failure", 0, 0, "", err)
			return nil, err, ExitInvalidInput
		}
		basisID = basisEntry.BackupID
		parentID = basisEntry.BackupID
		chainID = basisEntry.ChainID
		sequence = basisEntry.Sequence + 1
		baseBytes, err := o.resolveEntryBytes(cat, storageAdapter, basisEntry)
		if err != nil {
			o.notifySlack(backupID, "failure", 0, 0, "", err)
			return nil, fmt.Errorf("failed to resolve differential basis: %w", err), ExitBackupFailure
		}
		artifactBytes, err = backupdelta.EncodeDelta(baseBytes, currentDumpBytes)
		if err != nil {
			o.notifySlack(backupID, "failure", 0, 0, "", err)
			return nil, fmt.Errorf("failed to create differential delta: %w", err), ExitBackupFailure
		}
		artifactKind = "patch"
	}

	storedBytes, err := compressArtifactBytes(artifactBytes, strings.ToLower(o.cfg.Backup.Compress))
	if err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, err, ExitBackupFailure
	}

	hasher := sha256.New()
	tr := &trackingReader{r: bytes.NewReader(storedBytes), h: hasher}
	meta := storage.ArtifactMetadata{
		DBType:       o.cfg.DB.Type,
		BackupMode:   mode,
		ArtifactKind: artifactKind,
		Timestamp:    time.Now().Format("20060102_150405.000000000"),
		Compression:  strings.ToLower(o.cfg.Backup.Compress),
	}

	ref, err := storageAdapter.Write(tr, meta)
	if err != nil {
		o.notifySlack(backupID, "failure", 0, 0, "", err)
		return nil, fmt.Errorf("storage write failed: %w", err), ExitStorageFailure
	}

	checksumStr := hex.EncodeToString(hasher.Sum(nil))
	ref.Checksum = checksumStr
	ref.SizeBytes = tr.n

	entry := catalog.Entry{
		BackupID:       backupID,
		ChainID:        chainID,
		DBType:         o.cfg.DB.Type,
		Database:       o.cfg.DB.Database,
		Mode:           mode,
		ArtifactKind:   artifactKind,
		ArtifactURI:    ref.URI,
		BasisBackupID:  basisID,
		ParentBackupID: parentID,
		Sequence:       sequence,
		Compression:    strings.ToLower(o.cfg.Backup.Compress),
		Checksum:       checksumStr,
		SizeBytes:      tr.n,
		CreatedAt:      time.Now().UTC(),
	}

	cat.Add(entry)
	if err := o.saveCatalog(cat); err != nil {
		o.notifySlack(backupID, "failure", tr.n, 0, ref.URI, err)
		return nil, fmt.Errorf("failed to persist catalog entry: %w", err), ExitBackupFailure
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

	cat, err := o.loadCatalog()
	if err != nil {
		return nil, fmt.Errorf("failed to load backup catalog: %w", err), ExitRestoreFailure
	}

	target, ok := cat.FindByURI(o.cfg.Restore.BackupPath)
	if !ok {
		return nil, fmt.Errorf("backup path not found in catalog: %s", o.cfg.Restore.BackupPath), ExitRestoreFailure
	}

	restoredBytes, err := o.resolveEntryBytes(cat, storageAdapter, target)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct backup chain: %w", err), ExitRestoreFailure
	}

	stats, err := dbHandler.StreamRestore(restoreCtx, bytes.NewReader(restoredBytes))
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
