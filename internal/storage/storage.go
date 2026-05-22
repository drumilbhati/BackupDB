package storage

import (
	"io"
)

type ArtifactMetadata struct {
	DBType      string
	BackupMode  string
	Timestamp   string
	Checksum    string
	SizeBytes   int64
	Compression string
	Labels      map[string]string
}

type ArtifactRef struct {
	URI         string
	StorageType string
	Checksum    string
	SizeBytes   int64
}

type StorageAdapter interface {
	ValidateTarget() error
	Write(input io.Reader, meta ArtifactMetadata) (ArtifactRef, error)
	Read(ref ArtifactRef) (io.ReadCloser, error)
}
