package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/drumilbhati/BackupDB/internal/config"
)

type LocalAdapter struct {
	cfg config.StorageConfig
}

func NewLocalAdapter(cfg config.StorageConfig) StorageAdapter {
	return &LocalAdapter{cfg: cfg}
}

func (a *LocalAdapter) ValidateTarget() error {
	if a.cfg.LocalPath == "" {
		return fmt.Errorf("local path config is empty")
	}

	absPath, err := filepath.Abs(a.cfg.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to resolve local path: %w", err)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("failed to create local backup directory %s: %w", absPath, err)
	}

	// Verify write permission by creating and removing a temporary file
	tempFile, err := os.CreateTemp(absPath, ".write_test_")
	if err != nil {
		return fmt.Errorf("local path %s is not writable: %w", absPath, err)
	}
	tempFile.Close()
	os.Remove(tempFile.Name())

	return nil
}

func (a *LocalAdapter) Write(input io.Reader, meta ArtifactMetadata) (ArtifactRef, error) {
	if err := a.ValidateTarget(); err != nil {
		return ArtifactRef{}, err
	}

	ext := "sql"
	if meta.DBType == "mongodb" {
		ext = "archive"
	}

	filename := fmt.Sprintf("backup_%s.%s", meta.Timestamp, ext)
	if meta.Compression == "gzip" {
		filename += ".gz"
	} else if meta.Compression == "zstd" {
		filename += ".zst"
	}

	dirPath := filepath.Join(a.cfg.LocalPath, meta.DBType)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return ArtifactRef{}, fmt.Errorf("failed to create db-specific local storage directory: %w", err)
	}

	filePath := filepath.Join(dirPath, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("failed to create local backup file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, input)
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("failed to write data to local backup file: %w", err)
	}

	absPath, _ := filepath.Abs(filePath)

	return ArtifactRef{
		URI:         absPath,
		StorageType: "local",
		Checksum:    meta.Checksum,
		SizeBytes:   meta.SizeBytes,
	}, nil
}

func (a *LocalAdapter) Read(ref ArtifactRef) (io.ReadCloser, error) {
	file, err := os.Open(ref.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to open local backup file: %w", err)
	}
	return file, nil
}
