package postgres

import (
	"context"

	"api-hamy/internal/domain"
	"api-hamy/internal/repository/postgres/sqlcgen"
)

// AlbumRepository implements domain.AlbumRepository on Postgres.
// Every album read goes through the album_access view: a user without access
// gets ErrNotFound, never album data.
type AlbumRepository struct {
	q *sqlcgen.Queries
}

var _ domain.AlbumRepository = (*AlbumRepository)(nil)

func NewAlbumRepository(db sqlcgen.DBTX) *AlbumRepository {
	return &AlbumRepository{q: sqlcgen.New(db)}
}

func (r *AlbumRepository) Create(ctx context.Context, a *domain.Album) error {
	row, err := r.q.CreateAlbum(ctx, sqlcgen.CreateAlbumParams{
		OwnerID:     a.OwnerID,
		Title:       a.Title,
		Description: a.Description,
	})
	if err != nil {
		return mapErr(err)
	}

	*a = mapAlbum(row, string(domain.RoleOwner))
	return nil
}

func (r *AlbumRepository) GetForActor(ctx context.Context, albumID, actorID string) (*domain.Album, error) {
	row, err := r.q.GetAlbumForActor(ctx, sqlcgen.GetAlbumForActorParams{
		ID:      albumID,
		ActorID: actorID,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	a := mapAlbumForActorRow(row)
	return &a, nil
}

func (r *AlbumRepository) ListForActor(ctx context.Context, actorID string, f domain.ListFilter) ([]domain.Album, int64, error) {
	total, err := r.q.CountAlbumsForActor(ctx, sqlcgen.CountAlbumsForActorParams{
		ActorID: actorID,
		Search:  f.Search,
	})
	if err != nil {
		return nil, 0, mapErr(err)
	}

	rows, err := r.q.ListAlbumsForActor(ctx, sqlcgen.ListAlbumsForActorParams{
		ActorID: actorID,
		Search:  f.Search,
		Off:     int32(f.Offset()),
		Lim:     int32(f.PerPage),
	})
	if err != nil {
		return nil, 0, mapErr(err)
	}

	albums := make([]domain.Album, 0, len(rows))
	for _, row := range rows {
		albums = append(albums, mapAlbumListRow(row))
	}

	return albums, total, nil
}

func (r *AlbumRepository) Update(ctx context.Context, albumID, actorID string, title, description *string, clearDescription bool) (*domain.Album, error) {
	row, err := r.q.UpdateAlbum(ctx, sqlcgen.UpdateAlbumParams{
		Title:            title,
		ClearDescription: clearDescription,
		Description:      description,
		ID:               albumID,
		ActorID:          actorID,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	a := mapAlbum(row, string(domain.RoleOwner))
	return &a, nil
}

func (r *AlbumRepository) SetCover(ctx context.Context, albumID, actorID string, photoID *string) (*domain.Album, error) {
	row, err := r.q.SetAlbumCover(ctx, sqlcgen.SetAlbumCoverParams{
		CoverPhotoID: uuidValuePtr(photoID),
		ID:           albumID,
		ActorID:      actorID,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	a := mapAlbum(row, string(domain.RoleOwner))
	return &a, nil
}

func (r *AlbumRepository) Delete(ctx context.Context, albumID, actorID string) error {
	rows, err := r.q.SoftDeleteAlbum(ctx, sqlcgen.SoftDeleteAlbumParams{
		ID:      albumID,
		ActorID: actorID,
	})
	if err != nil {
		return mapErr(err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AlbumRepository) ClearCoverForPhoto(ctx context.Context, photoID string) error {
	if _, err := r.q.ClearAlbumCover(ctx, photoID); err != nil {
		return mapErr(err)
	}
	return nil
}

func (r *AlbumRepository) UpsertMember(ctx context.Context, m *domain.AlbumMember) (*domain.AlbumMember, error) {
	row, err := r.q.UpsertAlbumMember(ctx, sqlcgen.UpsertAlbumMemberParams{
		AlbumID:   m.AlbumID,
		UserID:    m.UserID,
		Role:      string(m.Role),
		Status:    string(m.Status),
		InvitedBy: uuidValuePtr(m.InvitedBy),
	})
	if err != nil {
		return nil, mapErr(err)
	}

	member := mapMember(row)
	return &member, nil
}

func (r *AlbumRepository) GetMember(ctx context.Context, albumID, userID string) (*domain.AlbumMember, error) {
	row, err := r.q.GetAlbumMember(ctx, sqlcgen.GetAlbumMemberParams{
		AlbumID: albumID,
		UserID:  userID,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	member := mapMember(row)
	return &member, nil
}

func (r *AlbumRepository) ListMembers(ctx context.Context, albumID, actorID string) ([]domain.AlbumMember, error) {
	rows, err := r.q.ListAlbumMembers(ctx, sqlcgen.ListAlbumMembersParams{
		ActorID: actorID,
		AlbumID: albumID,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	members := make([]domain.AlbumMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, mapMemberListRow(row))
	}

	return members, nil
}

func (r *AlbumRepository) ListInvitations(ctx context.Context, userID string) ([]domain.AlbumInvitation, error) {
	rows, err := r.q.ListInvitationsForUser(ctx, userID)
	if err != nil {
		return nil, mapErr(err)
	}

	invites := make([]domain.AlbumInvitation, 0, len(rows))
	for _, row := range rows {
		invites = append(invites, mapInvitationRow(row))
	}

	return invites, nil
}

func (r *AlbumRepository) ActivateMember(ctx context.Context, albumID, userID string) error {
	rows, err := r.q.ActivateAlbumMember(ctx, sqlcgen.ActivateAlbumMemberParams{
		AlbumID: albumID,
		UserID:  userID,
	})
	if err != nil {
		return mapErr(err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AlbumRepository) UpdateMemberRole(ctx context.Context, albumID, userID string, role domain.MemberRole) error {
	rows, err := r.q.UpdateAlbumMemberRole(ctx, sqlcgen.UpdateAlbumMemberRoleParams{
		Role:    string(role),
		AlbumID: albumID,
		UserID:  userID,
	})
	if err != nil {
		return mapErr(err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AlbumRepository) RevokeMember(ctx context.Context, albumID, userID string) error {
	rows, err := r.q.RevokeAlbumMember(ctx, sqlcgen.RevokeAlbumMemberParams{
		AlbumID: albumID,
		UserID:  userID,
	})
	if err != nil {
		return mapErr(err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}
