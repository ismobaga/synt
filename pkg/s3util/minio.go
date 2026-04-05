// Package s3util provides MinIO and other S3-compatible storage helpers.
package s3util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	defaultStorageProvider = "stub"
	defaultMinIOEndpoint   = "localhost:9000"
	defaultMinIOAccessKey  = "minioadmin"
	defaultMinIOSecretKey  = "minioadmin"
	defaultMinIOBucket     = "synt-assets"
	defaultMinIORegion     = "us-east-1"
)

// Config holds S3-compatible storage settings.
type Config struct {
	Endpoint         string
	AccessKey        string
	SecretKey        string
	Bucket           string
	Region           string
	PublicBaseURL    string
	UseSSL           bool
	AutoCreateBucket bool
}

// MinIOClient implements the Client interface using a MinIO/S3 backend.
type MinIOClient struct {
	client           *minio.Client
	bucket           string
	region           string
	publicBaseURL    string
	autoCreateBucket bool
}

// NewFromEnv constructs a storage client from environment variables.
func NewFromEnv() (Client, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_PROVIDER")))
	if provider == "" {
		if strings.TrimSpace(os.Getenv("MINIO_ENDPOINT")) != "" {
			provider = "minio"
		} else {
			provider = defaultStorageProvider
		}
	}

	switch provider {
	case "stub":
		baseURL := strings.TrimRight(getEnv("MINIO_PUBLIC_BASE_URL", "http://localhost:9000"), "/")
		return NewStubClient(fmt.Sprintf("%s/%s", baseURL, getEnv("MINIO_BUCKET", defaultMinIOBucket))), nil
	case "minio", "s3":
		return NewMinIOClient(Config{
			Endpoint:         getEnv("MINIO_ENDPOINT", defaultMinIOEndpoint),
			AccessKey:        getEnv("MINIO_ACCESS_KEY", defaultMinIOAccessKey),
			SecretKey:        getEnv("MINIO_SECRET_KEY", defaultMinIOSecretKey),
			Bucket:           getEnv("MINIO_BUCKET", defaultMinIOBucket),
			Region:           getEnv("MINIO_REGION", defaultMinIORegion),
			PublicBaseURL:    strings.TrimSpace(os.Getenv("MINIO_PUBLIC_BASE_URL")),
			UseSSL:           envBool("MINIO_USE_SSL", false),
			AutoCreateBucket: envBool("MINIO_AUTO_CREATE_BUCKET", true),
		})
	default:
		return nil, fmt.Errorf("unsupported storage provider %q", provider)
	}
}

// NewMinIOClient creates a MinIO-backed S3-compatible client.
func NewMinIOClient(cfg Config) (*MinIOClient, error) {
	endpoint, useSSL, err := normalizeEndpoint(cfg.Endpoint, cfg.UseSSL)
	if err != nil {
		return nil, err
	}

	bucket := strings.Trim(strings.TrimSpace(cfg.Bucket), "/")
	if bucket == "" {
		bucket = defaultMinIOBucket
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = defaultMinIORegion
	}

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	publicBaseURL := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")
	if publicBaseURL == "" {
		scheme := "http"
		if useSSL {
			scheme = "https"
		}
		publicBaseURL = fmt.Sprintf("%s://%s", scheme, endpoint)
	}

	return &MinIOClient{
		client:           mc,
		bucket:           bucket,
		region:           region,
		publicBaseURL:    publicBaseURL,
		autoCreateBucket: cfg.AutoCreateBucket,
	}, nil
}

// Upload stores data in the configured bucket and returns its public URL.
func (c *MinIOClient) Upload(ctx context.Context, objectPath string, r io.Reader, contentType string) (string, error) {
	objectPath = cleanObjectPath(objectPath)
	if objectPath == "" {
		return "", fmt.Errorf("object path is required")
	}
	if err := c.ensureBucket(ctx); err != nil {
		return "", err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read upload data: %w", err)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = c.client.PutObject(ctx, c.bucket, objectPath, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("upload %s: %w", objectPath, err)
	}
	return c.URL(objectPath), nil
}

// Download retrieves data at the given path.
func (c *MinIOClient) Download(ctx context.Context, objectPath string) (io.ReadCloser, error) {
	objectPath = cleanObjectPath(objectPath)
	if objectPath == "" {
		return nil, fmt.Errorf("object path is required")
	}

	obj, err := c.client.GetObject(ctx, c.bucket, objectPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", objectPath, err)
	}
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("download %s: %w", objectPath, err)
	}
	return obj, nil
}

// Delete removes an object.
func (c *MinIOClient) Delete(ctx context.Context, objectPath string) error {
	objectPath = cleanObjectPath(objectPath)
	if objectPath == "" {
		return fmt.Errorf("object path is required")
	}
	if err := c.client.RemoveObject(ctx, c.bucket, objectPath, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete %s: %w", objectPath, err)
	}
	return nil
}

// URL returns the access URL for a stored path.
func (c *MinIOClient) URL(objectPath string) string {
	base := strings.TrimRight(c.publicBaseURL, "/")
	objectPath = cleanObjectPath(objectPath)
	if objectPath == "" {
		return fmt.Sprintf("%s/%s", base, c.bucket)
	}
	return fmt.Sprintf("%s/%s/%s", base, c.bucket, objectPath)
}

func (c *MinIOClient) ensureBucket(ctx context.Context) error {
	if !c.autoCreateBucket {
		return nil
	}

	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket %s: %w", c.bucket, err)
	}
	if exists {
		return nil
	}

	if err := c.client.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{Region: c.region}); err != nil {
		exists, checkErr := c.client.BucketExists(ctx, c.bucket)
		if checkErr == nil && exists {
			return nil
		}
		return fmt.Errorf("create bucket %s: %w", c.bucket, err)
	}
	return nil
}

func normalizeEndpoint(raw string, useSSL bool) (string, bool, error) {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" {
		return "", false, fmt.Errorf("minio endpoint is required")
	}

	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", false, fmt.Errorf("parse endpoint %q: %w", raw, err)
		}
		if parsed.Host == "" {
			return "", false, fmt.Errorf("invalid endpoint %q", raw)
		}
		endpoint = parsed.Host
		switch strings.ToLower(parsed.Scheme) {
		case "https":
			useSSL = true
		case "http":
			useSSL = false
		}
	}

	return strings.TrimRight(endpoint, "/"), useSSL, nil
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func cleanObjectPath(objectPath string) string {
	return strings.TrimLeft(strings.TrimSpace(objectPath), "/")
}
