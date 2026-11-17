package domain

import "time"

// PhotoStatus is the lifecycle of a photo relative to MinIO:
// pending (upload URL issued), ready (object verified) or failed.
type PhotoStatus string

const (
	PhotoPending PhotoStatus = "pending"
	PhotoReady   PhotoStatus = "ready"
	PhotoFailed  PhotoStatus = "failed"
)

// Photo is image metadata; the bytes live in MinIO.
type Photo struct {
	ID               string
	AlbumID          string
	UploadedBy       *string
	Bucket           string
	ObjectKey        string
	OriginalFilename string
	ContentType      string
	SizeBytes        int64
	Width            *int32
	Height           *int32
	ChecksumSHA256   *string
	Status           PhotoStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// UploadTicket is what the client receives to upload directly to MinIO.
type UploadTicket struct {
	PhotoID   string
	UploadURL string
	ExpiresAt time.Time
}
