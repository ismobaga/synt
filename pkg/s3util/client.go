// Package s3util provides utilities for S3-compatible object storage.
package s3util

import (
	"context"
	"fmt"
	"io"
)

// Client is the interface for object storage operations.
type Client interface {
	// Upload stores data at the given path and returns the public URL.
	Upload(ctx context.Context, path string, r io.Reader, contentType string) (string, error)
	// Download retrieves data at the given path.
	Download(ctx context.Context, path string) (io.ReadCloser, error)
	// Delete removes an object.
	Delete(ctx context.Context, path string) error
	// URL returns the access URL for a stored path.
	URL(path string) string
}

// StubClient is a no-op storage client for testing.
type StubClient struct {
	BaseURL string
}

// NewStubClient creates a stub storage client.
func NewStubClient(baseURL string) *StubClient {
	return &StubClient{BaseURL: baseURL}
}

// Upload is a no-op that returns a predictable URL.
func (c *StubClient) Upload(_ context.Context, path string, _ io.Reader, _ string) (string, error) {
	return c.URL(path), nil
}

// Download returns an error indicating stub mode.
func (c *StubClient) Download(_ context.Context, path string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("stub: download not supported for %s", path)
}

// Delete is a no-op.
func (c *StubClient) Delete(_ context.Context, _ string) error {
	return nil
}

// URL returns the full URL for a path.
func (c *StubClient) URL(path string) string {
	return fmt.Sprintf("%s/%s", c.BaseURL, path)
}
