package admin

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"api-hamy/internal/domain"
	userservice "api-hamy/internal/service/user"
	"api-hamy/internal/testutil"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fixture struct {
	svc   *Service
	users *testutil.FakeUserRepo
	admin *domain.User
	user  *domain.User
}

func newFixture() *fixture {
	repo := testutil.NewFakeUserRepo()
	userSvc := userservice.New(repo, testLogger())
	svc := New(userSvc, testLogger())

	return &fixture{
		svc:   svc,
		users: repo,
		admin: repo.AddAdmin("Admin", "admin@example.com"),
		user:  repo.Add("Ana", "ana@example.com"),
	}
}

func TestListAndGet(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	users, total, err := f.svc.List(ctx, domain.ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 2 || len(users) != 2 {
		t.Fatalf("List() = %d users (total %d), want 2", len(users), total)
	}

	u, err := f.svc.Get(ctx, f.user.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if u.Email != "ana@example.com" {
		t.Fatalf("Get() email = %q", u.Email)
	}
}

func TestUpdateUser(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	name := "Ana María"
	email := " ANA.MARIA@Example.com "
	isAdmin := true

	u, err := f.svc.Update(ctx, f.admin.ID, f.user.ID, userservice.UpdateInput{
		Name:    &name,
		Email:   &email,
		IsAdmin: &isAdmin,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if u.Name != "Ana María" || u.Email != "ana.maria@example.com" || !u.IsAdmin {
		t.Fatalf("Update() = %+v, want normalized name/email and admin flag", u)
	}

	invalid := "x"
	if _, err := f.svc.Update(ctx, f.admin.ID, f.user.ID, userservice.UpdateInput{Name: &invalid}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Update(invalid name) error = %v, want ErrInvalid", err)
	}
}

func TestSelfProtection(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	notAdmin := false
	if _, err := f.svc.Update(ctx, f.admin.ID, f.admin.ID, userservice.UpdateInput{IsAdmin: &notAdmin}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Update(self-demote) error = %v, want ErrInvalid", err)
	}

	if err := f.svc.Delete(ctx, f.admin.ID, f.admin.ID); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Delete(self) error = %v, want ErrInvalid", err)
	}
}

func TestDeleteUser(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	if err := f.svc.Delete(ctx, f.admin.ID, f.user.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := f.svc.Get(ctx, f.user.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}
}
