package domain

import "time"

// MemberRole is what a user can do within an album.
type MemberRole string

const (
	RoleOwner  MemberRole = "owner"
	RoleEditor MemberRole = "editor"
	RoleViewer MemberRole = "viewer"
)

// IsValidRole reports whether the role can be assigned to an invited member.
func (r MemberRole) IsValidRole() bool {
	return r == RoleEditor || r == RoleViewer
}

// CanEdit reports whether the role can upload and delete photos.
func (r MemberRole) CanEdit() bool {
	return r == RoleOwner || r == RoleEditor
}

// IsOwner reports whether the role has full control of the album.
func (r MemberRole) IsOwner() bool {
	return r == RoleOwner
}

// MemberStatus is the state of an invitation or membership.
type MemberStatus string

const (
	StatusInvited MemberStatus = "invited"
	StatusActive  MemberStatus = "active"
	StatusRevoked MemberStatus = "revoked"
)

// Album is a photo album owned by a user.
type Album struct {
	ID           string
	OwnerID      string
	Title        string
	Description  *string
	CoverPhotoID *string
	AccessRole   MemberRole
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

// AlbumMember is the relationship between an album and an invited user.
type AlbumMember struct {
	AlbumID   string
	UserID    string
	Role      MemberRole
	Status    MemberStatus
	InvitedBy *string
	UserName  string
	UserEmail string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AlbumInvitation is a pending invitation as seen by the invited user.
type AlbumInvitation struct {
	AlbumID        string
	AlbumTitle     string
	UserID         string
	Role           MemberRole
	Status         MemberStatus
	InvitedByName  *string
	InvitedByEmail *string
	CreatedAt      time.Time
}
