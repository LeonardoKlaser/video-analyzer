// Package gcs wraps Google Cloud Storage operations needed by the analyzer:
// V4 signed PUT URLs and object deletion.
package gcs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/storage"
)

type Client struct {
	bucket string
	client *storage.Client
}

func New(ctx context.Context, bucket string) (*Client, error) {
	c, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	return &Client{bucket: bucket, client: c}, nil
}

func (c *Client) Close() error { return c.client.Close() }

// SignedPutURL returns a V4 signed URL that allows a PUT upload to objectKey
// using the given Content-Type. Valid for 15 minutes.
func (c *Client) SignedPutURL(objectKey, contentType string) (string, time.Time, error) {
	expires := time.Now().Add(15 * time.Minute)
	opts := &storage.SignedURLOptions{
		Scheme:      storage.SigningSchemeV4,
		Method:      "PUT",
		Expires:     expires,
		ContentType: contentType,
	}
	url, err := c.client.Bucket(c.bucket).SignedURL(objectKey, opts)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("SignedURL: %w", err)
	}
	return url, expires, nil
}

// Delete removes an object. Accepts either a gs://bucket/key URI or a plain key.
func (c *Client) Delete(ctx context.Context, uriOrKey string) error {
	key := uriOrKey
	if strings.HasPrefix(key, "gs://") {
		parts := strings.SplitN(strings.TrimPrefix(key, "gs://"), "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid gs uri: %s", uriOrKey)
		}
		if parts[0] != c.bucket {
			return fmt.Errorf("uri bucket %s does not match client bucket %s", parts[0], c.bucket)
		}
		key = parts[1]
	}
	return c.client.Bucket(c.bucket).Object(key).Delete(ctx)
}

// Bucket returns the bucket name (for building gs:// URIs).
func (c *Client) Bucket() string { return c.bucket }
