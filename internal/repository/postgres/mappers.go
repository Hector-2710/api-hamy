package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"api-hamy/internal/domain"
	"api-hamy/internal/repository/postgres/sqlcgen"
)

// Helpers that translate between sqlc-generated types and the domain.
// sqlc maps nullable uuid to pgtype.UUID and timestamptz to pgtype.Timestamptz.

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func uuidPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

func uuidValue(s string) pgtype.UUID {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func uuidValuePtr(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{}
	}
	return uuidValue(*s)
}

func mapUser(u sqlcgen.User) domain.User {
	return domain.User{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		DeletedAt: timePtr(u.DeletedAt),
		CreatedAt: u.CreatedAt.Time,
		UpdatedAt: u.UpdatedAt.Time,
	}
}

func mapAlbum(a sqlcgen.Album, role string) domain.Album {
	return domain.Album{
		ID:           a.ID,
		OwnerID:      a.OwnerID,
		Title:        a.Title,
		Description:  a.Description,
		CoverPhotoID: uuidPtr(a.CoverPhotoID),
		AccessRole:   domain.MemberRole(role),
		CreatedAt:    a.CreatedAt.Time,
		UpdatedAt:    a.UpdatedAt.Time,
		DeletedAt:    timePtr(a.DeletedAt),
	}
}

func mapAlbumForActorRow(a sqlcgen.GetAlbumForActorRow) domain.Album {
	return domain.Album{
		ID:           a.ID,
		OwnerID:      a.OwnerID,
		Title:        a.Title,
		Description:  a.Description,
		CoverPhotoID: uuidPtr(a.CoverPhotoID),
		AccessRole:   domain.MemberRole(a.AccessRole),
		CreatedAt:    a.CreatedAt.Time,
		UpdatedAt:    a.UpdatedAt.Time,
		DeletedAt:    timePtr(a.DeletedAt),
	}
}

func mapAlbumListRow(a sqlcgen.ListAlbumsForActorRow) domain.Album {
	return domain.Album{
		ID:           a.ID,
		OwnerID:      a.OwnerID,
		Title:        a.Title,
		Description:  a.Description,
		CoverPhotoID: uuidPtr(a.CoverPhotoID),
		AccessRole:   domain.MemberRole(a.AccessRole),
		CreatedAt:    a.CreatedAt.Time,
		UpdatedAt:    a.UpdatedAt.Time,
		DeletedAt:    timePtr(a.DeletedAt),
	}
}

func mapMember(m sqlcgen.AlbumMember) domain.AlbumMember {
	return domain.AlbumMember{
		AlbumID:   m.AlbumID,
		UserID:    m.UserID,
		Role:      domain.MemberRole(m.Role),
		Status:    domain.MemberStatus(m.Status),
		InvitedBy: uuidPtr(m.InvitedBy),
		CreatedAt: m.CreatedAt.Time,
		UpdatedAt: m.UpdatedAt.Time,
	}
}

func mapMemberListRow(m sqlcgen.ListAlbumMembersRow) domain.AlbumMember {
	return domain.AlbumMember{
		AlbumID:   m.AlbumID,
		UserID:    m.UserID,
		Role:      domain.MemberRole(m.Role),
		Status:    domain.MemberStatus(m.Status),
		InvitedBy: uuidPtr(m.InvitedBy),
		UserName:  m.UserName,
		UserEmail: m.UserEmail,
		CreatedAt: m.CreatedAt.Time,
		UpdatedAt: m.UpdatedAt.Time,
	}
}

func mapInvitationRow(i sqlcgen.ListInvitationsForUserRow) domain.AlbumInvitation {
	return domain.AlbumInvitation{
		AlbumID:        i.AlbumID,
		AlbumTitle:     i.AlbumTitle,
		UserID:         i.UserID,
		Role:           domain.MemberRole(i.Role),
		Status:         domain.MemberStatus(i.Status),
		InvitedByName:  i.InvitedByName,
		InvitedByEmail: i.InvitedByEmail,
		CreatedAt:      i.CreatedAt.Time,
	}
}

func mapPhoto(p sqlcgen.Photo) domain.Photo {
	return domain.Photo{
		ID:               p.ID,
		AlbumID:          p.AlbumID,
		UploadedBy:       uuidPtr(p.UploadedBy),
		Bucket:           p.Bucket,
		ObjectKey:        p.ObjectKey,
		OriginalFilename: p.OriginalFilename,
		ContentType:      p.ContentType,
		SizeBytes:        p.SizeBytes,
		Width:            p.Width,
		Height:           p.Height,
		ChecksumSHA256:   p.ChecksumSha256,
		Status:           domain.PhotoStatus(p.Status),
		CreatedAt:        p.CreatedAt.Time,
		UpdatedAt:        p.UpdatedAt.Time,
		DeletedAt:        timePtr(p.DeletedAt),
	}
}
