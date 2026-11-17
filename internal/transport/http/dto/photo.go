package dto

import (
	"time"

	"api-hamy/internal/domain"
)

// InitUploadRequest is the body of POST /api/v1/albums/:id/photos/uploads.
type InitUploadRequest struct {
	Filename    string `json:"filename" binding:"required,max=255"`
	ContentType string `json:"content_type" binding:"required"`
	SizeBytes   int64  `json:"size_bytes" binding:"required,gt=0"`
}

// CompleteUploadRequest is the body of POST /api/v1/albums/:id/photos/:photoId/complete.
type CompleteUploadRequest struct {
	Width  *int32 `json:"width" binding:"omitempty,gt=0"`
	Height *int32 `json:"height" binding:"omitempty,gt=0"`
}

// ListPhotosQuery holds the query params of GET /api/v1/albums/:id/photos.
type ListPhotosQuery struct {
	Page    int    `form:"page,default=1" binding:"min=1"`
	PerPage int    `form:"per_page,default=20" binding:"min=1,max=100"`
	Search  string `form:"search"`
}

// PhotoResponse is the JSON representation of a photo (without bytes).
type PhotoResponse struct {
	ID               string    `json:"id"`
	AlbumID          string    `json:"album_id"`
	UploadedBy       *string   `json:"uploaded_by"`
	OriginalFilename string    `json:"original_filename"`
	ContentType      string    `json:"content_type"`
	SizeBytes        int64     `json:"size_bytes"`
	Width            *int32    `json:"width"`
	Height           *int32    `json:"height"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

// NewPhotoResponse maps a domain photo to its JSON representation.
func NewPhotoResponse(p *domain.Photo) PhotoResponse {
	return PhotoResponse{
		ID:               p.ID,
		AlbumID:          p.AlbumID,
		UploadedBy:       p.UploadedBy,
		OriginalFilename: p.OriginalFilename,
		ContentType:      p.ContentType,
		SizeBytes:        p.SizeBytes,
		Width:            p.Width,
		Height:           p.Height,
		Status:           string(p.Status),
		CreatedAt:        p.CreatedAt,
	}
}

// NewPhotoResponses maps a list of domain photos.
func NewPhotoResponses(photos []domain.Photo) []PhotoResponse {
	out := make([]PhotoResponse, 0, len(photos))
	for i := range photos {
		out = append(out, NewPhotoResponse(&photos[i]))
	}
	return out
}

// UploadTicketResponse returns the presigned upload URL.
type UploadTicketResponse struct {
	PhotoID   string    `json:"photo_id"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DownloadURLResponse returns the presigned download URL.
type DownloadURLResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}
