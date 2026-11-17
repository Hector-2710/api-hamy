package domain

import (
	"context"
	"time"
)

// ListFilter groups pagination and search parameters.
type ListFilter struct {
	Page    int
	PerPage int
	Search  string
}

// Offset returns the query offset.
func (f ListFilter) Offset() int {
	if f.Page < 1 {
		return 0
	}
	return (f.Page - 1) * f.PerPage
}

// Normalize applies defaults and bounds to pagination.
func (f ListFilter) Normalize() ListFilter {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}
	if f.PerPage > 100 {
		f.PerPage = 100
	}
	return f
}

// UserRepository is the user persistence port.
type UserRepository interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context, f ListFilter) ([]User, int64, error)
	Update(ctx context.Context, u *User) error
	Delete(ctx context.Context, id string) error
}

// AlbumRepository is the album and membership persistence port.
type AlbumRepository interface {
	Create(ctx context.Context, a *Album) error
	GetForActor(ctx context.Context, albumID, actorID string) (*Album, error)
	ListForActor(ctx context.Context, actorID string, f ListFilter) ([]Album, int64, error)
	Update(ctx context.Context, albumID, actorID string, title, description *string, clearDescription bool) (*Album, error)
	SetCover(ctx context.Context, albumID, actorID string, photoID *string) (*Album, error)
	Delete(ctx context.Context, albumID, actorID string) error
	ClearCoverForPhoto(ctx context.Context, photoID string) error

	UpsertMember(ctx context.Context, m *AlbumMember) (*AlbumMember, error)
	GetMember(ctx context.Context, albumID, userID string) (*AlbumMember, error)
	ListMembers(ctx context.Context, albumID, actorID string) ([]AlbumMember, error)
	ListInvitations(ctx context.Context, userID string) ([]AlbumInvitation, error)
	ActivateMember(ctx context.Context, albumID, userID string) error
	UpdateMemberRole(ctx context.Context, albumID, userID string, role MemberRole) error
	RevokeMember(ctx context.Context, albumID, userID string) error
}

// PhotoRepository is the photo metadata persistence port.
type PhotoRepository interface {
	CreatePending(ctx context.Context, albumID, uploadedBy, bucket, filename, contentType string, size int64) (*Photo, error)
	GetForActor(ctx context.Context, photoID, actorID string) (*Photo, error)
	GetForEditor(ctx context.Context, photoID, actorID string) (*Photo, error)
	ListForActor(ctx context.Context, albumID, actorID string, f ListFilter) ([]Photo, int64, error)
	MarkReady(ctx context.Context, photoID string, width, height *int32, checksum *string) (*Photo, error)
	SoftDelete(ctx context.Context, photoID string) error
	// ListStalePending returns photos that were never confirmed, for a cleanup job.
	ListStalePending(ctx context.Context, before time.Time, limit int32) ([]Photo, error)
}
