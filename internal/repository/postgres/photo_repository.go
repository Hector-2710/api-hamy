package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"api-hamy/internal/domain"
	"api-hamy/internal/repository/postgres/sqlcgen"
)

// PhotoRepository implements domain.PhotoRepository on Postgres.
// Permissions are resolved in SQL through album_access; a caller without
// access gets ErrNotFound.
type PhotoRepository struct {
	q *sqlcgen.Queries
}

var _ domain.PhotoRepository = (*PhotoRepository)(nil)

func NewPhotoRepository(db sqlcgen.DBTX) *PhotoRepository {
	return &PhotoRepository{q: sqlcgen.New(db)}
}

func (r *PhotoRepository) CreatePending(ctx context.Context, albumID, uploadedBy, bucket, filename, contentType string, size int64) (*domain.Photo, error) {
	row, err := r.q.CreatePendingPhoto(ctx, sqlcgen.CreatePendingPhotoParams{
		AlbumID:          albumID,
		UploadedBy:       uploadedBy,
		Bucket:           bucket,
		OriginalFilename: filename,
		ContentType:      contentType,
		SizeBytes:        size,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	p := mapPhoto(row)
	return &p, nil
}

func (r *PhotoRepository) GetForActor(ctx context.Context, photoID, actorID string) (*domain.Photo, error) {
	row, err := r.q.GetPhotoForActor(ctx, sqlcgen.GetPhotoForActorParams{
		ActorID: actorID,
		ID:      photoID,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	p := mapPhoto(row)
	return &p, nil
}

func (r *PhotoRepository) GetForEditor(ctx context.Context, photoID, actorID string) (*domain.Photo, error) {
	row, err := r.q.GetPhotoForEditor(ctx, sqlcgen.GetPhotoForEditorParams{
		ActorID: actorID,
		ID:      photoID,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	p := mapPhoto(row)
	return &p, nil
}

func (r *PhotoRepository) ListForActor(ctx context.Context, albumID, actorID string, f domain.ListFilter) ([]domain.Photo, int64, error) {
	total, err := r.q.CountPhotosForActor(ctx, sqlcgen.CountPhotosForActorParams{
		ActorID: actorID,
		AlbumID: albumID,
		Search:  f.Search,
	})
	if err != nil {
		return nil, 0, mapErr(err)
	}

	rows, err := r.q.ListPhotosForActor(ctx, sqlcgen.ListPhotosForActorParams{
		ActorID: actorID,
		AlbumID: albumID,
		Search:  f.Search,
		Off:     int32(f.Offset()),
		Lim:     int32(f.PerPage),
	})
	if err != nil {
		return nil, 0, mapErr(err)
	}

	photos := make([]domain.Photo, 0, len(rows))
	for _, row := range rows {
		photos = append(photos, mapPhoto(row))
	}

	return photos, total, nil
}

func (r *PhotoRepository) MarkReady(ctx context.Context, photoID string, width, height *int32, checksum *string) (*domain.Photo, error) {
	row, err := r.q.MarkPhotoReady(ctx, sqlcgen.MarkPhotoReadyParams{
		Width:    width,
		Height:   height,
		Checksum: checksum,
		ID:       photoID,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	p := mapPhoto(row)
	return &p, nil
}

func (r *PhotoRepository) SoftDelete(ctx context.Context, photoID string) error {
	rows, err := r.q.SoftDeletePhoto(ctx, photoID)
	if err != nil {
		return mapErr(err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PhotoRepository) ListStalePending(ctx context.Context, before time.Time, limit int32) ([]domain.Photo, error) {
	rows, err := r.q.ListStalePendingPhotos(ctx, sqlcgen.ListStalePendingPhotosParams{
		Before: pgtype.Timestamptz{Time: before, Valid: true},
		Lim:    limit,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	photos := make([]domain.Photo, 0, len(rows))
	for _, row := range rows {
		photos = append(photos, mapPhoto(row))
	}

	return photos, nil
}
