package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/drumilbhati/BackupDB/internal/config"
)

func TestLocalAdapter(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "backupdb_local_test_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.StorageConfig{
		Type:      "local",
		LocalPath: tempDir,
	}

	adapter := NewLocalAdapter(cfg)

	// Test ValidateTarget
	if err := adapter.ValidateTarget(); err != nil {
		t.Errorf("ValidateTarget() failed: %v", err)
	}

	// Test Write
	data := []byte("hello database backup")
	meta := ArtifactMetadata{
		DBType:      "sqlite",
		BackupMode:  "full",
		Timestamp:   "20260522_120000",
		Compression: "none",
		Checksum:    "mockchecksum",
		SizeBytes:   int64(len(data)),
	}

	ref, err := adapter.Write(bytes.NewReader(data), meta)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	if ref.StorageType != "local" {
		t.Errorf("Expected StorageType local, got %s", ref.StorageType)
	}

	expectedPath := filepath.Join(tempDir, "sqlite", "backup_20260522_120000.sql")
	absExpectedPath, _ := filepath.Abs(expectedPath)
	if ref.URI != absExpectedPath {
		t.Errorf("Expected URI %s, got %s", absExpectedPath, ref.URI)
	}

	// Test Read
	rc, err := adapter.Read(ref)
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}
	defer rc.Close()

	readData, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Failed to read from reader: %v", err)
	}

	if string(readData) != string(data) {
		t.Errorf("Expected read data %s, got %s", string(data), string(readData))
	}
}
