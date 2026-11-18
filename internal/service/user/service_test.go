package user

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"api-hamy/internal/domain"
	"api-hamy/internal/testutil"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newFixture() (*Service, *testutil.FakeUserRepo) {
	repo := testutil.NewFakeUserRepo()
	return New(repo, testLogger()), repo
}

func TestCreateNormalizesAndValidates(t *testing.T) {
	svc, _ := newFixture()
	ctx := context.Background()

	u, err := svc.Create(ctx, CreateInput{Name: "  Ana  ", Email: " ANA@Example.COM "})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if u.Name != "Ana" || u.Email != "ana@example.com" {
		t.Fatalf("Create() = %q/%q, want Ana/ana@example.com", u.Name, u.Email)
	}
	if u.ID == "" {
		t.Fatal("Create() returned an empty ID")
	}

	cases := []struct {
		name  string
		user  string
		email string
		want  error
	}{
		{"short name", "A", "b@example.com", domain.ErrInvalid},
		{"invalid email", "Bea", "not-an-email", domain.ErrInvalid},
		{"duplicate email", "Bea", "ANA@example.com", domain.ErrConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(ctx, CreateInput{Name: tc.user, Email: tc.email})
			if !errors.Is(err, tc.want) {
				t.Fatalf("Create() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCreateStoresCredentials(t *testing.T) {
	svc, _ := newFixture()
	ctx := context.Background()

	u, err := svc.Create(ctx, CreateInput{
		Name:         "Admin",
		Email:        "admin@example.com",
		PasswordHash: "hash",
		IsAdmin:      true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if u.PasswordHash != "hash" || !u.IsAdmin {
		t.Fatalf("Create() = %+v, want password hash and admin flag", u)
	}
}

func TestUpdateIsPartial(t *testing.T) {
	svc, repo := newFixture()
	ctx := context.Background()

	u := repo.Add("Ana", "ana@example.com")
	repo.Add("Luis", "luis@example.com")

	name := "Ana María"
	updated, err := svc.Update(ctx, u.ID, UpdateInput{Name: &name})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "Ana María" || updated.Email != "ana@example.com" {
		t.Fatalf("Update() = %+v, want name changed and email kept", updated)
	}

	email := " ANA.MARIA@Example.com "
	updated, err = svc.Update(ctx, u.ID, UpdateInput{Email: &email})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Email != "ana.maria@example.com" {
		t.Fatalf("Update() email = %q, want normalized", updated.Email)
	}

	taken := "marta@example.com"
	repo.Add("Marta", taken)
	if _, err := svc.Update(ctx, u.ID, UpdateInput{Email: &taken}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("Update() error = %v, want ErrConflict", err)
	}

	invalid := "x"
	if _, err := svc.Update(ctx, u.ID, UpdateInput{Name: &invalid}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Update() error = %v, want ErrInvalid", err)
	}

	isAdmin := true
	updated, err = svc.Update(ctx, u.ID, UpdateInput{IsAdmin: &isAdmin})
	if err != nil {
		t.Fatalf("Update(admin) error = %v", err)
	}
	if !updated.IsAdmin {
		t.Fatal("Update(admin) did not set the admin flag")
	}
}

func TestDeleteAndGet(t *testing.T) {
	svc, repo := newFixture()
	ctx := context.Background()

	u := repo.Add("Ana", "ana@example.com")
	if err := svc.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := svc.Get(ctx, u.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(ctx, u.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Delete() twice error = %v, want ErrNotFound", err)
	}
}

func TestListPaginationAndSearch(t *testing.T) {
	svc, repo := newFixture()
	ctx := context.Background()

	repo.Add("Ana", "ana@example.com")
	repo.Add("Luis", "luis@example.com")
	repo.Add("Marta", "marta@example.com")

	items, total, err := svc.List(ctx, domain.ListFilter{Page: 1, PerPage: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 3 || len(items) != 2 {
		t.Fatalf("List() = %d items, total %d; want 2/3", len(items), total)
	}

	items, total, err = svc.List(ctx, domain.ListFilter{Search: "luis"})
	if err != nil {
		t.Fatalf("List(search) error = %v", err)
	}
	if total != 1 || items[0].Email != "luis@example.com" {
		t.Fatalf("List(search) = %+v, want only Luis", items)
	}
}
