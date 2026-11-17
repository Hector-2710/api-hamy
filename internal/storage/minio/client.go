// Package minio implements domain.BlobStore on top of MinIO (S3 compatible).
package minio

import (
	"context"
	"errors"
	"fmt"
	"time"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"api-hamy/internal/domain"
)

// Client talks to MinIO through two endpoints:
//   - internal: for API operations (Stat, Remove, bucket management).
//   - public:   for signing URLs consumed by the end client.
//
// Inside Docker the internal endpoint is "minio:9000" (not resolvable from the
// host), while presigned URLs must point to "localhost:9000".
type Client struct {
	internal *miniogo.Client
	public   *miniogo.Client
	bucket   string
	region   string
}

// New creates both clients. Connectivity is verified by EnsureBucket.
func New(endpoint, publicEndpoint, accessKey, secretKey, region, bucket string, useSSL, publicUseSSL bool) (*Client, error) {
	internal, err := newSDKClient(endpoint, accessKey, secretKey, region, useSSL)
	if err != nil {
		return nil, fmt.Errorf("internal minio: %w", err)
	}

	public, err := newSDKClient(publicEndpoint, accessKey, secretKey, region, publicUseSSL)
	if err != nil {
		return nil, fmt.Errorf("public minio: %w", err)
	}

	return &Client{internal: internal, public: public, bucket: bucket, region: region}, nil
}

func newSDKClient(endpoint, accessKey, secretKey, region string, useSSL bool) (*miniogo.Client, error) {
	if endpoint == "" {
		return nil, errors.New("empty endpoint")
	}
	return miniogo.New(endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
}

// EnsureBucket creates the bucket if it does not exist. It is idempotent.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.internal.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("checking bucket %q: %w", c.bucket, err)
	}
	if exists {
		return nil
	}

	if err := c.internal.MakeBucket(ctx, c.bucket, miniogo.MakeBucketOptions{Region: c.region}); err != nil {
		// Another process may have created it between the check and the make.
		exists, checkErr := c.internal.BucketExists(ctx, c.bucket)
		if checkErr == nil && exists {
			return nil
		}
		return fmt.Errorf("creating bucket %q: %w", c.bucket, err)
	}

	return nil
}

// PresignPut signs a URL to upload an object with PUT.
func (c *Client) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	u, err := c.public.PresignedPutObject(ctx, c.bucket, key, ttl)
	if err != nil {
		return "", fmt.Errorf("signing upload: %w", err)
	}
	return u.String(), nil
}

// PresignGet signs a URL to download an object.
func (c *Client) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := c.public.PresignedGetObject(ctx, c.bucket, key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("signing download: %w", err)
	}
	return u.String(), nil
}

// Stat returns object metadata or domain.ErrBlobNotFound.
func (c *Client) Stat(ctx context.Context, key string) (domain.BlobInfo, error) {
	info, err := c.internal.StatObject(ctx, c.bucket, key, miniogo.StatObjectOptions{})
	if err != nil {
		if miniogo.ToErrorResponse(err).Code == "NoSuchKey" {
			return domain.BlobInfo{}, domain.ErrBlobNotFound
		}
		return domain.BlobInfo{}, fmt.Errorf("stat object: %w", err)
	}

	return domain.BlobInfo{Size: info.Size, ContentType: info.ContentType}, nil
}

// Remove deletes the object. Deleting a missing object is not an error.
func (c *Client) Remove(ctx context.Context, key string) error {
	if err := c.internal.RemoveObject(ctx, c.bucket, key, miniogo.RemoveObjectOptions{}); err != nil {
		if miniogo.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil
		}
		return fmt.Errorf("removing object: %w", err)
	}
	return nil
}
