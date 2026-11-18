package dto

import (
	"time"

	"api-hamy/internal/domain"
)

// CreateAlbumRequest is the body of POST /api/v1/albums.
type CreateAlbumRequest struct {
	Title       string  `json:"title" binding:"required,max=150"`
	Description *string `json:"description" binding:"omitempty,max=2000"`
}

// UpdateAlbumRequest is the body of PATCH /api/v1/albums/:id (fields are optional).
// An empty description clears it.
type UpdateAlbumRequest struct {
	Title       *string `json:"title" binding:"omitempty,min=1,max=150"`
	Description *string `json:"description" binding:"omitempty,max=2000"`
}

// SetCoverRequest is the body of PUT /api/v1/albums/:id/cover.
type SetCoverRequest struct {
	PhotoID string `json:"photo_id" binding:"required,uuid"`
}

// ListAlbumsQuery holds the query params of GET /api/v1/albums.
type ListAlbumsQuery struct {
	Page    int    `form:"page,default=1" binding:"min=1"`
	PerPage int    `form:"per_page,default=20" binding:"min=1,max=100"`
	Search  string `form:"search"`
}

// AlbumResponse is the JSON representation of an album.
type AlbumResponse struct {
	ID           string    `json:"id"`
	OwnerID      string    `json:"owner_id"`
	Title        string    `json:"title"`
	Description  *string   `json:"description"`
	CoverPhotoID *string   `json:"cover_photo_id"`
	AccessRole   string    `json:"access_role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewAlbumResponse maps a domain album to its JSON representation.
func NewAlbumResponse(a *domain.Album) AlbumResponse {
	return AlbumResponse{
		ID:           a.ID,
		OwnerID:      a.OwnerID,
		Title:        a.Title,
		Description:  a.Description,
		CoverPhotoID: a.CoverPhotoID,
		AccessRole:   string(a.AccessRole),
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

// NewAlbumResponses maps a list of domain albums.
func NewAlbumResponses(albums []domain.Album) []AlbumResponse {
	out := make([]AlbumResponse, 0, len(albums))
	for i := range albums {
		out = append(out, NewAlbumResponse(&albums[i]))
	}
	return out
}
