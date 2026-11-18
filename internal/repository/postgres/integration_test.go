//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"api-hamy/internal/domain"
	"api-hamy/internal/repository/postgres"
)

// TestRepositoryFlow runs against a real Postgres with migrations applied.
// Set DB_TEST_DSN (make test-integration does it for you); otherwise it skips.
func TestRepositoryFlow(t *testing.T) {
	dsn := os.Getenv("DB_TEST_DSN")
	if dsn == "" {
		t.Skip("DB_TEST_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, dsn, 4)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// Test emails start with itest- so cleanup cannot touch real data.
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM users WHERE email LIKE 'itest-%'`); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})

	users := postgres.NewUserRepository(pool)
	albums := postgres.NewAlbumRepository(pool)
	photos := postgres.NewPhotoRepository(pool)

	suffix := uuid.NewString()
	ana := &domain.User{Name: "Ana", Email: "itest-ana-" + suffix + "@example.com", PasswordHash: "itest-hash", IsAdmin: true}
	luis := &domain.User{Name: "Luis", Email: "itest-luis-" + suffix + "@example.com"}
	marta := &domain.User{Name: "Marta", Email: "itest-marta-" + suffix + "@example.com"}
	for _, u := range []*domain.User{ana, luis, marta} {
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("create user %s: %v", u.Email, err)
		}
	}

	stored, err := users.GetByEmail(ctx, strings.ToUpper(ana.Email))
	if err != nil {
		t.Fatalf("GetByEmail is not case-insensitive: %v", err)
	}
	if stored.PasswordHash != "itest-hash" || !stored.IsAdmin {
		t.Fatalf("stored auth fields = %q/%v, want password hash and admin flag", stored.PasswordHash, stored.IsAdmin)
	}

	if err := users.Create(ctx, &domain.User{Name: "Dup", Email: ana.Email}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate email error = %v, want ErrConflict", err)
	}

	// Credentials and the admin flag are mutable through the repository.
	ana.PasswordHash = "itest-hash-rotated"
	ana.IsAdmin = false
	if err := users.Update(ctx, ana); err != nil {
		t.Fatalf("update user auth fields: %v", err)
	}
	stored, err = users.GetByID(ctx, ana.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if stored.PasswordHash != "itest-hash-rotated" || stored.IsAdmin {
		t.Fatalf("updated auth fields = %q/%v, want rotated hash and no admin", stored.PasswordHash, stored.IsAdmin)
	}

	album := &domain.Album{OwnerID: ana.ID, Title: "ITest " + suffix}
	if err := albums.Create(ctx, album); err != nil {
		t.Fatalf("create album: %v", err)
	}
	if album.AccessRole != domain.RoleOwner {
		t.Fatalf("created album role = %q, want owner", album.AccessRole)
	}

	// Invited members do not have access until they accept.
	if _, err := albums.GetForActor(ctx, album.ID, luis.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("invited GetForActor error = %v, want ErrNotFound", err)
	}
	if _, err := albums.UpsertMember(ctx, &domain.AlbumMember{
		AlbumID: album.ID, UserID: luis.ID, Role: domain.RoleViewer, Status: domain.StatusInvited, InvitedBy: &ana.ID,
	}); err != nil {
		t.Fatalf("UpsertMember: %v", err)
	}
	if _, err := albums.GetForActor(ctx, album.ID, luis.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("invited GetForActor error = %v, want ErrNotFound", err)
	}

	invites, err := albums.ListInvitations(ctx, luis.ID)
	if err != nil {
		t.Fatalf("ListInvitations: %v", err)
	}
	if len(invites) != 1 || invites[0].AlbumTitle != album.Title {
		t.Fatalf("invitations = %+v, want the album", invites)
	}

	if err := albums.ActivateMember(ctx, album.ID, luis.ID); err != nil {
		t.Fatalf("ActivateMember: %v", err)
	}
	member, err := albums.GetForActor(ctx, album.ID, luis.ID)
	if err != nil {
		t.Fatalf("active GetForActor: %v", err)
	}
	if member.AccessRole != domain.RoleViewer {
		t.Fatalf("active role = %q, want viewer", member.AccessRole)
	}

	// The permissions view hides the album from outsiders.
	if _, err := albums.GetForActor(ctx, album.ID, marta.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("outsider GetForActor error = %v, want ErrNotFound", err)
	}
	for _, user := range []*domain.User{ana, luis, marta} {
		list, _, err := albums.ListForActor(ctx, user.ID, domain.ListFilter{Page: 1, PerPage: 50})
		if err != nil {
			t.Fatalf("ListForActor: %v", err)
		}
		want := 0
		if user.ID != marta.ID {
			want = 1
		}
		if len(list) != want {
			t.Fatalf("ListForActor(%s) = %d albums, want %d", user.Email, len(list), want)
		}
	}

	// Photos: only editors can create pending rows.
	if _, err := photos.CreatePending(ctx, album.ID, luis.ID, "bucket", "viewer.png", "image/png", 10); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("viewer CreatePending error = %v, want ErrNotFound", err)
	}
	pending, err := photos.CreatePending(ctx, album.ID, ana.ID, "bucket", "photo.png", "image/png", 10)
	if err != nil {
		t.Fatalf("CreatePending: %v", err)
	}
	if pending.Status != domain.PhotoPending || pending.ObjectKey == "" {
		t.Fatalf("pending photo = %+v", pending)
	}

	if _, err := photos.GetForEditor(ctx, pending.ID, luis.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("viewer GetForEditor error = %v, want ErrNotFound", err)
	}
	if _, err := photos.GetForActor(ctx, pending.ID, luis.ID); err != nil {
		t.Fatalf("viewer GetForActor: %v", err)
	}

	ready, err := photos.MarkReady(ctx, pending.ID, nil, nil, nil)
	if err != nil {
		t.Fatalf("MarkReady: %v", err)
	}
	if ready.Status != domain.PhotoReady {
		t.Fatalf("ready status = %q, want ready", ready.Status)
	}

	list, total, err := photos.ListForActor(ctx, album.ID, luis.ID, domain.ListFilter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("ListForActor photos: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != pending.ID {
		t.Fatalf("photo list = %+v (total %d), want the ready photo", list, total)
	}

	// Cover: only a ready photo from the album.
	covered, err := albums.SetCover(ctx, album.ID, ana.ID, &pending.ID)
	if err != nil {
		t.Fatalf("SetCover: %v", err)
	}
	if covered.CoverPhotoID == nil || *covered.CoverPhotoID != pending.ID {
		t.Fatalf("cover = %v, want %s", covered.CoverPhotoID, pending.ID)
	}

	// Soft delete hides the photo and clears the cover.
	if err := photos.SoftDelete(ctx, pending.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, err := photos.GetForActor(ctx, pending.ID, ana.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetForActor after delete error = %v, want ErrNotFound", err)
	}
	if err := albums.ClearCoverForPhoto(ctx, pending.ID); err != nil {
		t.Fatalf("ClearCoverForPhoto: %v", err)
	}

	// Owner-only administration and soft delete.
	if err := albums.Delete(ctx, album.ID, luis.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("member Delete error = %v, want ErrNotFound", err)
	}
	if err := albums.Delete(ctx, album.ID, ana.ID); err != nil {
		t.Fatalf("owner Delete: %v", err)
	}
	if _, err := albums.GetForActor(ctx, album.ID, ana.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetForActor after delete error = %v, want ErrNotFound", err)
	}

	// Soft delete is idempotent from the API's perspective.
	if err := users.Delete(ctx, ana.ID); err != nil {
		t.Fatalf("Delete user: %v", err)
	}
	if _, err := users.GetByID(ctx, ana.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetByID after delete error = %v, want ErrNotFound", err)
	}
}

// TestReadySchema asserts that migrations were applied; it gives a clear error
// instead of a confusing SQL failure when the test DB is empty.
func TestReadySchema(t *testing.T) {
	dsn := os.Getenv("DB_TEST_DSN")
	if dsn == "" {
		t.Skip("DB_TEST_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.views WHERE table_name = 'album_access')`).Scan(&exists); err != nil {
		t.Fatalf("query album_access: %v", err)
	}
	if !exists {
		t.Fatal("album_access view not found; run migrations first (make migrate-up)")
	}

	var authColumns int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'users' AND column_name IN ('password_hash', 'is_admin')`).Scan(&authColumns); err != nil {
		t.Fatalf("query users auth columns: %v", err)
	}
	if authColumns != 2 {
		t.Fatal("users.password_hash/is_admin not found; run migrations first (make migrate-up)")
	}
}
