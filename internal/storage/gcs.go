package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/drumilbhati/BackupDB/internal/config"
	"google.golang.org/api/option"
)

type GCSAdapter struct {
	cfg config.StorageConfig
}

func NewGCSAdapter(cfg config.StorageConfig) StorageAdapter {
	return &GCSAdapter{cfg: cfg}
}

func (a *GCSAdapter) getClient() (*storage.Client, error) {
	ctx := context.Background()
	var opts []option.ClientOption

	if a.cfg.GCSCredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(a.cfg.GCSCredentialsFile))
	}

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}
	return client, nil
}

func (a *GCSAdapter) ValidateTarget() error {
	if a.cfg.Bucket == "" {
		return fmt.Errorf("gcs bucket is required")
	}

	client, err := a.getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	bucket := client.Bucket(a.cfg.Bucket)
	_, err = bucket.Attrs(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to validate GCS bucket %s: %w", a.cfg.Bucket, err)
	}

	return nil
}

func (a *GCSAdapter) Write(input io.Reader, meta ArtifactMetadata) (ArtifactRef, error) {
	client, err := a.getClient()
	if err != nil {
		return ArtifactRef{}, err
	}
	defer client.Close()

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

	objectName := path.Join(a.cfg.Prefix, meta.DBType, filename)
	bucket := client.Bucket(a.cfg.Bucket)
	obj := bucket.Object(objectName)

	w := obj.NewWriter(context.TODO())
	w.ContentType = "application/octet-stream"

	if _, err := io.Copy(w, input); err != nil {
		_ = w.Close()
		return ArtifactRef{}, fmt.Errorf("failed to write GCS object %s: %w", objectName, err)
	}

	if err := w.Close(); err != nil {
		return ArtifactRef{}, fmt.Errorf("failed to finalize GCS upload: %w", err)
	}

	uri := fmt.Sprintf("gs://%s/%s", a.cfg.Bucket, objectName)

	return ArtifactRef{
		URI:         uri,
		StorageType: "gcs",
		Checksum:    meta.Checksum,
		SizeBytes:   meta.SizeBytes,
	}, nil
}

func (a *GCSAdapter) Read(ref ArtifactRef) (io.ReadCloser, error) {
	client, err := a.getClient()
	if err != nil {
		return nil, err
	}

	bucketName, objectName, err := parseGCSURI(ref.URI)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)

	reader, err := obj.NewReader(context.TODO())
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to read GCS object: %w", err)
	}

	return &gcsReadCloser{
		rc:     reader,
		client: client,
	}, nil
}

type gcsReadCloser struct {
	rc     io.ReadCloser
	client *storage.Client
}

func (g *gcsReadCloser) Read(p []byte) (n int, err error) {
	return g.rc.Read(p)
}

func (g *gcsReadCloser) Close() error {
	err := g.rc.Close()
	_ = g.client.Close()
	return err
}

func parseGCSURI(uri string) (string, string, error) {
	if !strings.HasPrefix(uri, "gs://") {
		return "", "", fmt.Errorf("invalid GCS URI: %s", uri)
	}
	parts := strings.SplitN(uri[5:], "/", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GCS URI: %s (missing object path)", uri)
	}
	return parts[0], parts[1], nil
}
