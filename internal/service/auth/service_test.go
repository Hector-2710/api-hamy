package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"api-hamy/internal/domain"
	userservice "api-hamy/internal/service/user"
	"api-hamy/internal/testutil"
	"api-hamy/internal/token"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fixture struct {
	svc    *Service
	users  *testutil.FakeUserRepo
	tokens *token.Manager
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	manager, err := token.NewManager("test-secret-with-at-least-32-characters", "test", time.Hour)
	if err != nil {
		t.Fatalf("token.NewManager() error = %v", err)
	}

	repo := testutil.NewFakeUserRepo()
	users := userservice.New(repo, testLogger())
	svc := New(users, manager, testLogger())

	return &fixture{svc: svc, users: repo, tokens: manager}
}

func TestRegisterAndAuthenticate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	session, err := f.svc.Register(ctx, "Ana", " ANA@Example.com ", "supersecret")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if session.AccessToken == "" || session.ExpiresAt.Before(time.Now()) {
		t.Fatalf("Register() session = %+v, want a valid token", session)
	}
	if session.User.Email != "ana@example.com" || session.User.IsAdmin {
		t.Fatalf("Register() user = %+v, want normalized non-admin", session.User)
	}
	if session.User.PasswordHash == "" || session.User.PasswordHash == "supersecret" {
		t.Fatal("Register() did not hash the password")
	}
	if bcrypt.CompareHashAndPassword([]byte(session.User.PasswordHash), []byte("supersecret")) != nil {
		t.Fatal("Register() stored a hash that does not match the password")
	}

	authenticated, err := f.svc.Authenticate(ctx, session.AccessToken)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if authenticated.ID != session.User.ID {
		t.Fatalf("Authenticate() user = %s, want %s", authenticated.ID, session.User.ID)
	}
}

func TestRegisterValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Register(ctx, "Ana", "ana@example.com", "short"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Register(short password) error = %v, want ErrInvalid", err)
	}
	if _, err := f.svc.Register(ctx, "Ana", "ana@example.com", strings.Repeat("x", 73)); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Register(long password) error = %v, want ErrInvalid", err)
	}
	if _, err := f.svc.Register(ctx, "Ana", "not-an-email", "supersecret"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Register(invalid email) error = %v, want ErrInvalid", err)
	}
	if _, err := f.svc.Register(ctx, "A", "ana@example.com", "supersecret"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Register(invalid name) error = %v, want ErrInvalid", err)
	}

	if _, err := f.svc.Register(ctx, "Ana", "ana@example.com", "supersecret"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if _, err := f.svc.Register(ctx, "Otra", "ANA@example.com", "supersecret"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("Register(duplicate) error = %v, want ErrConflict", err)
	}
}

func TestLogin(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Register(ctx, "Ana", "ana@example.com", "supersecret"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	session, err := f.svc.Login(ctx, "ANA@example.com", "supersecret")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if session.AccessToken == "" {
		t.Fatal("Login() returned no token")
	}

	if _, err := f.svc.Login(ctx, "ana@example.com", "wrong-password"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("Login(wrong password) error = %v, want ErrUnauthorized", err)
	}
	unknown, err := f.svc.Login(ctx, "nobody@example.com", "supersecret")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("Login(unknown email) error = %v, want ErrUnauthorized", err)
	}
	if unknown != nil {
		t.Fatalf("Login(unknown email) session = %+v, want nil", unknown)
	}
	if !strings.Contains(err.Error(), "invalid email or password") {
		t.Fatalf("Login(unknown email) message = %q, want a generic message", err)
	}

	// Phase-0 users have no password and must not be able to log in.
	f.users.Add("Legacy", "legacy@example.com")
	if _, err := f.svc.Login(ctx, "legacy@example.com", ""); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("Login(legacy user) error = %v, want ErrUnauthorized", err)
	}
}

func TestAuthenticateRejectsInvalidState(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Authenticate(ctx, "garbage"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("Authenticate(garbage) error = %v, want ErrUnauthorized", err)
	}

	u := f.users.Add("Ana", "ana@example.com")
	raw, _, err := f.tokens.Issue(u.ID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if _, err := f.svc.Authenticate(ctx, raw); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if err := f.users.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := f.svc.Authenticate(ctx, raw); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("Authenticate(deleted user) error = %v, want ErrUnauthorized", err)
	}
}
