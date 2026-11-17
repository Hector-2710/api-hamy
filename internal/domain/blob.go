package domain

import (
	"context"
	"errors"
	"time"
)

// ErrBlobNotFound indicates that the object does not exist in storage.
var ErrBlobNotFound = errors.New("object not found in storage")

// BlobInfo is the minimum object metadata the domain needs.
type BlobInfo struct {
	Size        int64
	ContentType string
}

// BlobStore is the object storage port. Implementations live in internal/storage.
type BlobStore interface {
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Stat returns object metadata or ErrBlobNotFound.
	Stat(ctx context.Context, key string) (BlobInfo, error)
	// Remove deletes the object. Deleting a missing object is not an error.
	Remove(ctx context.Context, key string) error
}
