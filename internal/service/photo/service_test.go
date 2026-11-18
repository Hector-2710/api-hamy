package photo

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"api-hamy/internal/domain"
	"api-hamy/internal/testutil"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fixture struct {
	svc    *Service
	albums *testutil.FakeAlbumRepo
	photos *testutil.FakePhotoRepo
	blobs  *testutil.FakeBlobStore
	ana    *domain.User
	luis   *domain.User
	marta  *domain.User
	album  *domain.Album
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	users := testutil.NewFakeUserRepo()
	ana := users.Add("Ana", "ana@example.com")
	luis := users.Add("Luis", "luis@example.com")
	marta := users.Add("Marta", "marta@example.com")

	albums := testutil.NewFakeAlbumRepo(users)
	photos := testutil.NewFakePhotoRepo(albums)
	blobs := testutil.NewFakeBlobStore()
	albums.PhotoLookup = func(photoID string) (string, bool, bool) {
		p, ok := photos.Get(photoID)
		if !ok {
			return "", false, false
		}
		return p.AlbumID, p.Status == domain.PhotoReady, true
	}

	ctx := context.Background()
	album := &domain.Album{OwnerID: ana.ID, Title: "Álbum"}
	if err := albums.Create(ctx, album); err != nil {
		t.Fatalf("Create album error = %v", err)
	}
	if _, err := albums.UpsertMember(ctx, &domain.AlbumMember{
		AlbumID: album.ID, UserID: luis.ID, Role: domain.RoleViewer, Status: domain.StatusActive,
	}); err != nil {
		t.Fatalf("UpsertMember() error = %v", err)
	}
	if _, err := albums.UpsertMember(ctx, &domain.AlbumMember{
		AlbumID: album.ID, UserID: marta.ID, Role: domain.RoleEditor, Status: domain.StatusActive,
	}); err != nil {
		t.Fatalf("UpsertMember() error = %v", err)
	}

	svc := New(photos, albums, blobs, "test-bucket", 15*time.Minute, testLogger())

	return &fixture{
		svc:    svc,
		albums: albums,
		photos: photos,
		blobs:  blobs,
		ana:    ana,
		luis:   luis,
		marta:  marta,
		album:  album,
	}
}

func TestInitUploadValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	cases := []struct {
		name        string
		actor       string
		filename    string
		contentType string
		size        int64
		want        error
	}{
		{"empty filename", f.ana.ID, " ", "image/png", 10, domain.ErrInvalid},
		{"bad content type", f.ana.ID, "a.txt", "text/plain", 10, domain.ErrInvalid},
		{"zero size", f.ana.ID, "a.png", "image/png", 0, domain.ErrInvalid},
		{"too large", f.ana.ID, "a.png", "image/png", 26 << 20, domain.ErrInvalid},
		{"viewer", f.luis.ID, "a.png", "image/png", 10, domain.ErrForbidden},
		{"outsider", f.ana.ID + "x", "a.png", "image/png", 10, domain.ErrNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.InitUpload(ctx, tc.actor, f.album.ID, tc.filename, tc.contentType, tc.size)
			if !errors.Is(err, tc.want) {
				t.Fatalf("InitUpload() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestInitUploadCreatesPendingPhoto(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	ticket, err := f.svc.InitUpload(ctx, f.marta.ID, f.album.ID, "foto.png", "image/png", 1234)
	if err != nil {
		t.Fatalf("InitUpload() error = %v", err)
	}
	if ticket.PhotoID == "" || ticket.UploadURL == "" || ticket.ExpiresAt.Before(time.Now()) {
		t.Fatalf("InitUpload() ticket = %+v, want complete ticket", ticket)
	}

	p, ok := f.photos.Get(ticket.PhotoID)
	if !ok {
		t.Fatal("InitUpload() did not create the photo")
	}
	if p.Status != domain.PhotoPending || p.Bucket != "test-bucket" || p.SizeBytes != 1234 {
		t.Fatalf("photo = %+v, want pending in test-bucket with size 1234", p)
	}
	if p.CreatedAt.IsZero() {
		t.Fatal("photo has no timestamp")
	}
}

func TestCompleteUpload(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	ticket, err := f.svc.InitUpload(ctx, f.marta.ID, f.album.ID, "foto.png", "image/png", 4)
	if err != nil {
		t.Fatalf("InitUpload() error = %v", err)
	}
	key, _ := f.photos.ObjectKey(ticket.PhotoID)

	// Nothing was uploaded yet.
	if _, err := f.svc.Complete(ctx, f.marta.ID, f.album.ID, ticket.PhotoID, nil, nil); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Complete() without object error = %v, want ErrInvalid", err)
	}

	// Uploaded size does not match the declared size.
	f.blobs.Put(key, domain.BlobInfo{Size: 999, ContentType: "image/png"})
	if _, err := f.svc.Complete(ctx, f.marta.ID, f.album.ID, ticket.PhotoID, nil, nil); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Complete() size mismatch error = %v, want ErrInvalid", err)
	}

	f.blobs.Put(key, domain.BlobInfo{Size: 4, ContentType: "image/png"})
	width, height := int32(10), int32(20)
	p, err := f.svc.Complete(ctx, f.marta.ID, f.album.ID, ticket.PhotoID, &width, &height)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if p.Status != domain.PhotoReady || p.Width == nil || *p.Width != width || p.Height == nil || *p.Height != height {
		t.Fatalf("Complete() photo = %+v, want ready with dimensions", p)
	}

	// Idempotent: a second call returns the ready photo.
	p, err = f.svc.Complete(ctx, f.marta.ID, f.album.ID, ticket.PhotoID, nil, nil)
	if err != nil || p.Status != domain.PhotoReady {
		t.Fatalf("Complete() twice = %+v, %v; want ready", p, err)
	}

	// Wrong album id is a 404.
	if _, err := f.svc.Complete(ctx, f.marta.ID, f.ana.ID, ticket.PhotoID, nil, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Complete() wrong album error = %v, want ErrNotFound", err)
	}

	// A viewer cannot complete someone else's upload.
	other, err := f.svc.InitUpload(ctx, f.ana.ID, f.album.ID, "otra.png", "image/png", 4)
	if err != nil {
		t.Fatalf("InitUpload() error = %v", err)
	}
	otherKey, _ := f.photos.ObjectKey(other.PhotoID)
	f.blobs.Put(otherKey, domain.BlobInfo{Size: 4})
	if _, err := f.svc.Complete(ctx, f.luis.ID, f.album.ID, other.PhotoID, nil, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("viewer Complete() error = %v, want ErrNotFound", err)
	}
}

func TestCompleteRejectsInvalidDimensions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	ticket, err := f.svc.InitUpload(ctx, f.marta.ID, f.album.ID, "foto.png", "image/png", 4)
	if err != nil {
		t.Fatalf("InitUpload() error = %v", err)
	}
	key, _ := f.photos.ObjectKey(ticket.PhotoID)
	f.blobs.Put(key, domain.BlobInfo{Size: 4})

	zero := int32(0)
	if _, err := f.svc.Complete(ctx, f.marta.ID, f.album.ID, ticket.PhotoID, &zero, nil); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Complete() invalid width error = %v, want ErrInvalid", err)
	}
}

func TestListAndDownload(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	ticket, err := f.svc.InitUpload(ctx, f.marta.ID, f.album.ID, "foto.png", "image/png", 4)
	if err != nil {
		t.Fatalf("InitUpload() error = %v", err)
	}

	// Pending photos are not listed and cannot be downloaded.
	items, total, err := f.svc.List(ctx, f.luis.ID, f.album.ID, domain.ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("List() = %d photos, want 0", total)
	}
	if _, _, err := f.svc.DownloadURL(ctx, f.luis.ID, ticket.PhotoID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("DownloadURL(pending) error = %v, want ErrConflict", err)
	}

	key, _ := f.photos.ObjectKey(ticket.PhotoID)
	f.blobs.Put(key, domain.BlobInfo{Size: 4})
	if _, err := f.svc.Complete(ctx, f.marta.ID, f.album.ID, ticket.PhotoID, nil, nil); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	items, total, err = f.svc.List(ctx, f.luis.ID, f.album.ID, domain.ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Status != domain.PhotoReady {
		t.Fatalf("List() = %+v, want one ready photo", items)
	}

	url, expiresAt, err := f.svc.DownloadURL(ctx, f.luis.ID, ticket.PhotoID)
	if err != nil {
		t.Fatalf("DownloadURL() error = %v", err)
	}
	if url == "" || expiresAt.Before(time.Now()) {
		t.Fatalf("DownloadURL() = %q, %v; want a valid signed URL", url, expiresAt)
	}

	// Outsiders get a 404, not a 403.
	if _, _, err := f.svc.DownloadURL(ctx, f.ana.ID+"x", ticket.PhotoID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("outsider DownloadURL() error = %v, want ErrNotFound", err)
	}
}

func TestDeleteRemovesObjectAndCover(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	ticket, err := f.svc.InitUpload(ctx, f.marta.ID, f.album.ID, "foto.png", "image/png", 4)
	if err != nil {
		t.Fatalf("InitUpload() error = %v", err)
	}
	key, _ := f.photos.ObjectKey(ticket.PhotoID)
	f.blobs.Put(key, domain.BlobInfo{Size: 4})
	if _, err := f.svc.Complete(ctx, f.marta.ID, f.album.ID, ticket.PhotoID, nil, nil); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if _, err := f.albums.SetCover(ctx, f.album.ID, f.ana.ID, &ticket.PhotoID); err != nil {
		t.Fatalf("SetCover() error = %v", err)
	}

	if err := f.svc.Delete(ctx, f.marta.ID, ticket.PhotoID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if f.blobs.Has(key) {
		t.Fatal("Delete() did not remove the object from storage")
	}
	if p, ok := f.photos.Get(ticket.PhotoID); !ok || p.DeletedAt == nil {
		t.Fatalf("photo = %+v, want soft-deleted", p)
	}
	a, err := f.albums.GetForActor(ctx, f.album.ID, f.ana.ID)
	if err != nil {
		t.Fatalf("GetForActor() error = %v", err)
	}
	if a.CoverPhotoID != nil {
		t.Fatalf("cover = %v, want nil after deleting the cover photo", a.CoverPhotoID)
	}

	// The uploader keeps delete rights even after being downgraded to viewer.
	second, err := f.svc.InitUpload(ctx, f.marta.ID, f.album.ID, "dos.png", "image/png", 4)
	if err != nil {
		t.Fatalf("InitUpload() error = %v", err)
	}
	if err := f.albums.UpdateMemberRole(ctx, f.album.ID, f.marta.ID, domain.RoleViewer); err != nil {
		t.Fatalf("UpdateMemberRole() error = %v", err)
	}
	if err := f.svc.Delete(ctx, f.marta.ID, second.PhotoID); err != nil {
		t.Fatalf("Delete() as former editor error = %v", err)
	}

	// A viewer cannot delete a photo they did not upload.
	third, err := f.svc.InitUpload(ctx, f.ana.ID, f.album.ID, "tres.png", "image/png", 4)
	if err != nil {
		t.Fatalf("InitUpload() error = %v", err)
	}
	if err := f.svc.Delete(ctx, f.luis.ID, third.PhotoID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("viewer Delete() error = %v, want ErrNotFound", err)
	}
}
