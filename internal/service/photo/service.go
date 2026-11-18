package photo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"api-hamy/internal/domain"
)

const (
	maxFilenameLen = 255
	maxUploadBytes = 25 << 20 // 25 MiB
)

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
	"image/heic": true,
	"image/avif": true,
}

// Service implements the direct-to-MinIO upload flow with presigned URLs.
type Service struct {
	photos domain.PhotoRepository
	albums domain.AlbumRepository
	blobs  domain.BlobStore
	bucket string
	ttl    time.Duration
	log    *slog.Logger
}

func New(photos domain.PhotoRepository, albums domain.AlbumRepository, blobs domain.BlobStore, bucket string, ttl time.Duration, log *slog.Logger) *Service {
	return &Service{
		photos: photos,
		albums: albums,
		blobs:  blobs,
		bucket: bucket,
		ttl:    ttl,
		log:    log,
	}
}

// InitUpload validates permissions and returns a presigned URL to upload the photo.
func (s *Service) InitUpload(ctx context.Context, actorID, albumID, filename, contentType string, size int64) (*domain.UploadTicket, error) {
	filename = strings.TrimSpace(filename)
	contentType = strings.ToLower(strings.TrimSpace(contentType))

	if filename == "" || utf8.RuneCountInString(filename) > maxFilenameLen {
		return nil, fmt.Errorf("%w: invalid filename", domain.ErrInvalid)
	}
	if !allowedImageTypes[contentType] {
		return nil, fmt.Errorf("%w: file type not allowed", domain.ErrInvalid)
	}
	if size <= 0 || size > maxUploadBytes {
		return nil, fmt.Errorf("%w: size must be between 1 and %d bytes", domain.ErrInvalid, maxUploadBytes)
	}

	if err := s.requireEditor(ctx, actorID, albumID); err != nil {
		return nil, err
	}

	p, err := s.photos.CreatePending(ctx, albumID, actorID, s.bucket, filename, contentType, size)
	if err != nil {
		return nil, err
	}

	url, err := s.blobs.PresignPut(ctx, p.ObjectKey, contentType, s.ttl)
	if err != nil {
		return nil, err
	}

	return &domain.UploadTicket{
		PhotoID:   p.ID,
		UploadURL: url,
		ExpiresAt: time.Now().Add(s.ttl),
	}, nil
}

// Complete verifies that the object exists in MinIO and marks the photo as ready.
func (s *Service) Complete(ctx context.Context, actorID, albumID, photoID string, width, height *int32) (*domain.Photo, error) {
	p, err := s.photos.GetForEditor(ctx, photoID, actorID)
	if err != nil {
		return nil, err
	}
	if p.AlbumID != albumID {
		return nil, domain.ErrNotFound
	}

	// Idempotent: return the current state if it is already ready.
	if p.Status == domain.PhotoReady {
		return p, nil
	}
	if p.Status != domain.PhotoPending {
		return nil, fmt.Errorf("%w: photo is not in pending state", domain.ErrConflict)
	}

	if width != nil && *width <= 0 {
		return nil, fmt.Errorf("%w: invalid width", domain.ErrInvalid)
	}
	if height != nil && *height <= 0 {
		return nil, fmt.Errorf("%w: invalid height", domain.ErrInvalid)
	}

	info, err := s.blobs.Stat(ctx, p.ObjectKey)
	if err != nil {
		if errors.Is(err, domain.ErrBlobNotFound) {
			return nil, fmt.Errorf("%w: file was not uploaded to MinIO", domain.ErrInvalid)
		}
		return nil, err
	}

	if info.Size != p.SizeBytes {
		return nil, fmt.Errorf("%w: uploaded size (%d) does not match the declared size (%d)", domain.ErrInvalid, info.Size, p.SizeBytes)
	}

	updated, err := s.photos.MarkReady(ctx, photoID, width, height, nil)
	if err != nil {
		return nil, err
	}

	s.log.Info("photo confirmed", "photo_id", photoID, "album_id", albumID, "size", info.Size)
	return updated, nil
}

// List returns the ready photos of an album for any active member.
func (s *Service) List(ctx context.Context, actorID, albumID string, f domain.ListFilter) ([]domain.Photo, int64, error) {
	if _, err := s.albums.GetForActor(ctx, albumID, actorID); err != nil {
		return nil, 0, err
	}

	f = f.Normalize()
	f.Search = strings.TrimSpace(f.Search)

	return s.photos.ListForActor(ctx, albumID, actorID, f)
}

// DownloadURL returns a presigned download URL for an active member.
func (s *Service) DownloadURL(ctx context.Context, actorID, photoID string) (string, time.Time, error) {
	p, err := s.photos.GetForActor(ctx, photoID, actorID)
	if err != nil {
		return "", time.Time{}, err
	}
	if p.Status != domain.PhotoReady {
		return "", time.Time{}, fmt.Errorf("%w: photo is not available yet", domain.ErrConflict)
	}

	url, err := s.blobs.PresignGet(ctx, p.ObjectKey, s.ttl)
	if err != nil {
		return "", time.Time{}, err
	}

	return url, time.Now().Add(s.ttl), nil
}

// Delete soft-deletes the metadata and removes the object from MinIO.
// MinIO failures are logged and left for cleanup instead of failing the request.
func (s *Service) Delete(ctx context.Context, actorID, photoID string) error {
	p, err := s.photos.GetForEditor(ctx, photoID, actorID)
	if err != nil {
		return err
	}

	if err := s.photos.SoftDelete(ctx, photoID); err != nil {
		return err
	}

	if err := s.albums.ClearCoverForPhoto(ctx, photoID); err != nil {
		s.log.Warn("could not clear the cover", "photo_id", photoID, "err", err)
	}

	if err := s.blobs.Remove(ctx, p.ObjectKey); err != nil {
		s.log.Warn("could not remove the object from MinIO", "photo_id", photoID, "key", p.ObjectKey, "err", err)
	}

	s.log.Info("photo deleted", "photo_id", photoID, "actor_id", actorID)
	return nil
}

func (s *Service) requireEditor(ctx context.Context, actorID, albumID string) error {
	a, err := s.albums.GetForActor(ctx, albumID, actorID)
	if err != nil {
		return err
	}
	if !a.AccessRole.CanEdit() {
		return domain.ErrForbidden
	}
	return nil
}
