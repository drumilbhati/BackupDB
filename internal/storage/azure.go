package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/drumilbhati/BackupDB/internal/config"
)

type AzureAdapter struct {
	cfg config.StorageConfig
}

func NewAzureAdapter(cfg config.StorageConfig) StorageAdapter {
	return &AzureAdapter{cfg: cfg}
}

func (a *AzureAdapter) getClient() (*azblob.Client, error) {
	if a.cfg.AzureAccountName == "" || a.cfg.AzureAccountKey == "" {
		return nil, fmt.Errorf("azure account name and key are required")
	}

	credential, err := azblob.NewSharedKeyCredential(a.cfg.AzureAccountName, a.cfg.AzureAccountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credentials: %w", err)
	}

	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", a.cfg.AzureAccountName)
	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client: %w", err)
	}

	return client, nil
}

func (a *AzureAdapter) ValidateTarget() error {
	if a.cfg.Container == "" {
		return fmt.Errorf("azure container name is required")
	}

	credential, err := azblob.NewSharedKeyCredential(a.cfg.AzureAccountName, a.cfg.AzureAccountKey)
	if err != nil {
		return err
	}

	containerURL := fmt.Sprintf("https://%s.blob.core.windows.net/%s", a.cfg.AzureAccountName, a.cfg.Container)
	containerClient, err := container.NewClientWithSharedKeyCredential(containerURL, credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create Azure container client: %w", err)
	}

	_, err = containerClient.GetProperties(context.TODO(), nil)
	if err != nil {
		return fmt.Errorf("failed to validate Azure container %s: %w", a.cfg.Container, err)
	}

	return nil
}

func (a *AzureAdapter) Write(input io.Reader, meta ArtifactMetadata) (ArtifactRef, error) {
	client, err := a.getClient()
	if err != nil {
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

	blobName := path.Join(a.cfg.Prefix, meta.DBType, filename)

	_, err = client.UploadStream(context.TODO(), a.cfg.Container, blobName, input, nil)
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("failed to upload stream to Azure Blob %s: %w", blobName, err)
	}

	uri := fmt.Sprintf("azure://%s/%s", a.cfg.Container, blobName)

	return ArtifactRef{
		URI:         uri,
		StorageType: "azure",
		Checksum:    meta.Checksum,
		SizeBytes:   meta.SizeBytes,
	}, nil
}

func (a *AzureAdapter) Read(ref ArtifactRef) (io.ReadCloser, error) {
	client, err := a.getClient()
	if err != nil {
		return nil, err
	}

	containerName, blobName, err := parseAzureURI(ref.URI)
	if err != nil {
		return nil, err
	}

	resp, err := client.DownloadStream(context.TODO(), containerName, blobName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to download stream from Azure Blob: %w", err)
	}

	return resp.Body, nil
}

func parseAzureURI(uri string) (string, string, error) {
	if !strings.HasPrefix(uri, "azure://") {
		return "", "", fmt.Errorf("invalid Azure URI: %s", uri)
	}
	parts := strings.SplitN(uri[8:], "/", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid Azure URI: %s (missing blob path)", uri)
	}
	return parts[0], parts[1], nil
}
