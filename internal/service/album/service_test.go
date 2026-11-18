package album

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"api-hamy/internal/domain"
	"api-hamy/internal/testutil"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fixture struct {
	svc     *Service
	users   *testutil.FakeUserRepo
	albums  *testutil.FakeAlbumRepo
	photos  *testutil.FakePhotoRepo
	ana     *domain.User
	luis    *domain.User
	marta   *domain.User
	album   *domain.Album
	albumID string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	users := testutil.NewFakeUserRepo()
	ana := users.Add("Ana", "ana@example.com")
	luis := users.Add("Luis", "luis@example.com")
	marta := users.Add("Marta", "marta@example.com")

	albums := testutil.NewFakeAlbumRepo(users)
	photos := testutil.NewFakePhotoRepo(albums)
	albums.PhotoLookup = func(photoID string) (string, bool, bool) {
		p, ok := photos.Get(photoID)
		if !ok {
			return "", false, false
		}
		return p.AlbumID, p.Status == domain.PhotoReady, true
	}

	svc := New(albums, users, testLogger())
	album, err := svc.Create(context.Background(), ana.ID, "Viaje a la playa", nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	return &fixture{
		svc:     svc,
		users:   users,
		albums:  albums,
		photos:  photos,
		ana:     ana,
		luis:    luis,
		marta:   marta,
		album:   album,
		albumID: album.ID,
	}
}

func TestCreateValidatesTitle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Create(ctx, f.ana.ID, "   ", nil); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Create() empty title error = %v, want ErrInvalid", err)
	}
	if _, err := f.svc.Create(ctx, f.ana.ID, strings.Repeat("x", 151), nil); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Create() long title error = %v, want ErrInvalid", err)
	}
}

func TestGetRequiresActiveAccess(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Get(ctx, f.marta.ID, f.albumID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("outsider Get() error = %v, want ErrNotFound", err)
	}

	if _, err := f.svc.Invite(ctx, f.ana.ID, f.albumID, "", f.luis.Email, domain.RoleViewer); err != nil {
		t.Fatalf("Invite() error = %v", err)
	}

	// An invited user is not a member yet.
	if _, err := f.svc.Get(ctx, f.luis.ID, f.albumID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("invited Get() error = %v, want ErrNotFound", err)
	}

	if _, err := f.svc.Accept(ctx, f.luis.ID, f.albumID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}

	a, err := f.svc.Get(ctx, f.luis.ID, f.albumID)
	if err != nil {
		t.Fatalf("Get() after accept error = %v", err)
	}
	if a.AccessRole != domain.RoleViewer {
		t.Fatalf("Get() role = %q, want viewer", a.AccessRole)
	}
}

func TestPermissionMatrix(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Invite(ctx, f.ana.ID, f.albumID, "", f.luis.Email, domain.RoleViewer); err != nil {
		t.Fatalf("Invite() error = %v", err)
	}
	if _, err := f.svc.Accept(ctx, f.luis.ID, f.albumID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}

	// Viewer: read only.
	if _, err := f.svc.ListMembers(ctx, f.luis.ID, f.albumID); err != nil {
		t.Fatalf("viewer ListMembers() error = %v", err)
	}

	title := "Nuevo título"
	if _, err := f.svc.Update(ctx, f.luis.ID, f.albumID, &title, nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("viewer Update() error = %v, want ErrForbidden", err)
	}
	if err := f.svc.Delete(ctx, f.luis.ID, f.albumID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("viewer Delete() error = %v, want ErrForbidden", err)
	}
	if _, err := f.svc.SetCover(ctx, f.luis.ID, f.albumID, nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("viewer SetCover() error = %v, want ErrForbidden", err)
	}
	if _, err := f.svc.Invite(ctx, f.luis.ID, f.albumID, "", f.marta.Email, domain.RoleViewer); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("viewer Invite() error = %v, want ErrForbidden", err)
	}

	// Non-member: the album does not exist.
	if _, err := f.svc.Update(ctx, f.marta.ID, f.albumID, &title, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("outsider Update() error = %v, want ErrNotFound", err)
	}

	// Editor: still cannot administer the album.
	if err := f.svc.ChangeRole(ctx, f.ana.ID, f.albumID, f.luis.ID, domain.RoleEditor); err != nil {
		t.Fatalf("ChangeRole() error = %v", err)
	}
	if _, err := f.svc.Update(ctx, f.luis.ID, f.albumID, &title, nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("editor Update() error = %v, want ErrForbidden", err)
	}

	// Owner: full control.
	updated, err := f.svc.Update(ctx, f.ana.ID, f.albumID, &title, nil)
	if err != nil {
		t.Fatalf("owner Update() error = %v", err)
	}
	if updated.Title != title {
		t.Fatalf("owner Update() title = %q, want %q", updated.Title, title)
	}
}

func TestInviteRules(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	_, err := f.svc.Invite(ctx, f.ana.ID, f.albumID, "", "nobody@example.com", domain.RoleViewer)
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Invite(unknown email) error = %v, want ErrInvalid", err)
	}
	if !strings.Contains(err.Error(), "must have an account") {
		t.Fatalf("Invite(unknown email) message = %q, want account hint", err)
	}

	if _, err := f.svc.Invite(ctx, f.ana.ID, f.albumID, f.ana.ID, "", domain.RoleViewer); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Invite(self) error = %v, want ErrInvalid", err)
	}
	if _, err := f.svc.Invite(ctx, f.ana.ID, f.albumID, f.luis.ID, "", domain.RoleEditor); err != nil {
		t.Fatalf("Invite(by id) error = %v", err)
	}

	if err := f.svc.ChangeRole(ctx, f.ana.ID, f.albumID, f.ana.ID, domain.RoleViewer); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("ChangeRole(owner) error = %v, want ErrInvalid", err)
	}
	if err := f.svc.ChangeRole(ctx, f.ana.ID, f.albumID, f.luis.ID, "admin"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("ChangeRole(invalid) error = %v, want ErrInvalid", err)
	}
	if err := f.svc.RemoveMember(ctx, f.ana.ID, f.albumID, f.ana.ID); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("RemoveMember(owner) error = %v, want ErrInvalid", err)
	}
}

func TestAcceptIsIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Invite(ctx, f.ana.ID, f.albumID, f.luis.ID, "", domain.RoleViewer); err != nil {
		t.Fatalf("Invite() error = %v", err)
	}
	if _, err := f.svc.Accept(ctx, f.luis.ID, f.albumID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	member, err := f.svc.Accept(ctx, f.luis.ID, f.albumID)
	if err != nil {
		t.Fatalf("Accept() twice error = %v", err)
	}
	if member.Status != domain.StatusActive {
		t.Fatalf("Accept() status = %q, want active", member.Status)
	}
}

func TestLeaveAlbum(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Invite(ctx, f.ana.ID, f.albumID, f.luis.ID, "", domain.RoleViewer); err != nil {
		t.Fatalf("Invite() error = %v", err)
	}
	if _, err := f.svc.Accept(ctx, f.luis.ID, f.albumID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}

	if err := f.svc.RemoveMember(ctx, f.luis.ID, f.albumID, f.luis.ID); err != nil {
		t.Fatalf("leave album error = %v", err)
	}
	if _, err := f.svc.Get(ctx, f.luis.ID, f.albumID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get() after leaving error = %v, want ErrNotFound", err)
	}
}

func TestSetCoverRequiresReadyPhotoFromTheAlbum(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	pending, err := f.photos.CreatePending(ctx, f.albumID, f.ana.ID, "bucket", "p.png", "image/png", 10)
	if err != nil {
		t.Fatalf("CreatePending() error = %v", err)
	}
	if _, err := f.svc.SetCover(ctx, f.ana.ID, f.albumID, &pending.ID); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("SetCover(pending) error = %v, want ErrInvalid", err)
	}

	if _, err := f.photos.MarkReady(ctx, pending.ID, nil, nil, nil); err != nil {
		t.Fatalf("MarkReady() error = %v", err)
	}
	a, err := f.svc.SetCover(ctx, f.ana.ID, f.albumID, &pending.ID)
	if err != nil {
		t.Fatalf("SetCover(ready) error = %v", err)
	}
	if a.CoverPhotoID == nil || *a.CoverPhotoID != pending.ID {
		t.Fatalf("SetCover() cover = %v, want %s", a.CoverPhotoID, pending.ID)
	}

	otherAlbum, err := f.svc.Create(ctx, f.ana.ID, "Otro", nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	foreign, err := f.photos.CreatePending(ctx, otherAlbum.ID, f.ana.ID, "bucket", "q.png", "image/png", 10)
	if err != nil {
		t.Fatalf("CreatePending() error = %v", err)
	}
	if _, err := f.photos.MarkReady(ctx, foreign.ID, nil, nil, nil); err != nil {
		t.Fatalf("MarkReady() error = %v", err)
	}
	if _, err := f.svc.SetCover(ctx, f.ana.ID, f.albumID, &foreign.ID); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("SetCover(foreign) error = %v, want ErrInvalid", err)
	}

	if _, err := f.svc.SetCover(ctx, f.ana.ID, f.albumID, nil); err != nil {
		t.Fatalf("SetCover(clear) error = %v", err)
	}
}

func TestListAndInvitations(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Invite(ctx, f.ana.ID, f.albumID, "", f.luis.Email, domain.RoleViewer); err != nil {
		t.Fatalf("Invite() error = %v", err)
	}

	invites, err := f.svc.ListInvitations(ctx, f.luis.ID)
	if err != nil {
		t.Fatalf("ListInvitations() error = %v", err)
	}
	if len(invites) != 1 || invites[0].AlbumTitle != f.album.Title {
		t.Fatalf("ListInvitations() = %+v, want the album", invites)
	}
	if invites[0].InvitedByName == nil || *invites[0].InvitedByName != f.ana.Name {
		t.Fatalf("ListInvitations() inviter = %v, want Ana", invites[0].InvitedByName)
	}

	// Non-members do not see the album in their list.
	albums, total, err := f.svc.List(ctx, f.luis.ID, domain.ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 0 || len(albums) != 0 {
		t.Fatalf("List(invited) = %d albums, want 0", total)
	}

	if _, err := f.svc.Accept(ctx, f.luis.ID, f.albumID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	albums, total, err = f.svc.List(ctx, f.luis.ID, domain.ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 || albums[0].AccessRole != domain.RoleViewer {
		t.Fatalf("List(active) = %+v, want one viewer album", albums)
	}
}

func TestDeleteAlbumHidesItFromMembers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Invite(ctx, f.ana.ID, f.albumID, f.luis.ID, "", domain.RoleEditor); err != nil {
		t.Fatalf("Invite() error = %v", err)
	}
	if _, err := f.svc.Accept(ctx, f.luis.ID, f.albumID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}

	if err := f.svc.Delete(ctx, f.ana.ID, f.albumID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := f.svc.Get(ctx, f.ana.ID, f.albumID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("owner Get() after delete error = %v, want ErrNotFound", err)
	}
	if _, err := f.svc.Get(ctx, f.luis.ID, f.albumID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("member Get() after delete error = %v, want ErrNotFound", err)
	}
}
