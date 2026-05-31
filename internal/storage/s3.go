package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/drumilbhati/BackupDB/internal/config"
)

type S3Adapter struct {
	cfg config.StorageConfig
}

func NewS3Adapter(cfg config.StorageConfig) StorageAdapter {
	return &S3Adapter{cfg: cfg}
}

func (a *S3Adapter) getClient() (*s3.Client, error) {
	var opts []func(*awsconfig.LoadOptions) error

	if a.cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(a.cfg.Region))
	}
	if a.cfg.AccessKey != "" && a.cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(a.cfg.AccessKey, a.cfg.SecretKey, ""),
		))
	}

	sdkCfg, err := awsconfig.LoadDefaultConfig(context.TODO(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if a.cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(a.cfg.Endpoint)
		})
	}

	return s3.NewFromConfig(sdkCfg, s3Opts...), nil
}

func (a *S3Adapter) ValidateTarget() error {
	if a.cfg.Bucket == "" {
		return fmt.Errorf("s3 bucket is required")
	}

	client, err := a.getClient()
	if err != nil {
		return err
	}

	_, err = client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(a.cfg.Bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to validate S3 bucket %s: %w", a.cfg.Bucket, err)
	}

	return nil
}

func (a *S3Adapter) Write(input io.Reader, meta ArtifactMetadata) (ArtifactRef, error) {
	client, err := a.getClient()
	if err != nil {
		return ArtifactRef{}, err
	}

	ext := "sql"
	if meta.DBType == "mongodb" {
		ext = "archive"
	}
	if meta.ArtifactKind == "patch" {
		ext = "patch"
	}

	filename := fmt.Sprintf("backup_%s.%s", meta.Timestamp, ext)
	if meta.Compression == "gzip" {
		filename += ".gz"
	} else if meta.Compression == "zstd" {
		filename += ".zst"
	}

	key := path.Join(a.cfg.Prefix, meta.DBType, filename)

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(a.cfg.Bucket),
		Key:    aws.String(key),
		Body:   input,
	})
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("failed to upload backup to S3 key %s: %w", key, err)
	}

	uri := fmt.Sprintf("s3://%s/%s", a.cfg.Bucket, key)

	return ArtifactRef{
		URI:         uri,
		StorageType: "s3",
		Checksum:    meta.Checksum,
		SizeBytes:   meta.SizeBytes,
	}, nil
}

func (a *S3Adapter) Read(ref ArtifactRef) (io.ReadCloser, error) {
	client, err := a.getClient()
	if err != nil {
		return nil, err
	}

	bucket, key, err := parseS3URI(ref.URI)
	if err != nil {
		return nil, err
	}

	output, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch object from S3: %w", err)
	}

	return output.Body, nil
}

func parseS3URI(uri string) (string, string, error) {
	var bucket, key string
	_, err := fmt.Sscanf(uri, "s3://%s", &bucket)
	if err != nil {
		return "", "", fmt.Errorf("invalid S3 URI %s: %w", uri, err)
	}
	// split bucket and key
	idx := strings.Index(uri[5:], "/")
	if idx == -1 {
		return "", "", fmt.Errorf("invalid S3 URI %s: missing key", uri)
	}
	bucket = uri[5 : 5+idx]
	key = uri[5+idx+1:]
	return bucket, key, nil
}
