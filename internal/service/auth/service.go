// Package auth implements registration, login and access-token validation.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"

	"api-hamy/internal/domain"
	userservice "api-hamy/internal/service/user"
)

const (
	minPasswordLen = 8
	// bcrypt refuses inputs longer than 72 bytes; reject them explicitly.
	maxPasswordBytes = 72
)

// TokenManager is the token port used by the service.
type TokenManager interface {
	Issue(userID string) (string, time.Time, error)
	Parse(raw string) (string, error)
}

// Session is the result of a successful register/login.
type Session struct {
	User        *domain.User
	AccessToken string
	ExpiresAt   time.Time
}

// Service implements the authentication use cases.
type Service struct {
	users  *userservice.Service
	tokens TokenManager
	log    *slog.Logger
}

func New(users *userservice.Service, tokens TokenManager, log *slog.Logger) *Service {
	return &Service{users: users, tokens: tokens, log: log}
}

// dummyHash keeps the "unknown email" path as slow as a wrong password, so
// login does not leak whether an account exists through timing.
var dummyHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcrypt.DefaultCost)
	if err != nil {
		panic("auth: could not build dummy password hash: " + err.Error())
	}
	return hash
}()

// Register creates an account and returns an authenticated session.
func (s *Service) Register(ctx context.Context, name, email, password string) (*Session, error) {
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u, err := s.users.Create(ctx, userservice.CreateInput{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
	})
	if err != nil {
		return nil, err
	}

	return s.issue(u)
}

// Login verifies the credentials and returns an authenticated session.
func (s *Service) Login(ctx context.Context, email, password string) (*Session, error) {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Burn the same bcrypt work as a real comparison.
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
			return nil, invalidCredentials()
		}
		return nil, err
	}

	if u.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, invalidCredentials()
	}

	return s.issue(u)
}

// Authenticate validates the token and reloads the user from the database.
// Reloading on every request makes deleted users and permission changes take
// effect immediately.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (*domain.User, error) {
	userID, err := s.tokens.Parse(rawToken)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid or expired token", domain.ErrUnauthorized)
	}

	u, err := s.users.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("%w: user no longer exists", domain.ErrUnauthorized)
		}
		return nil, err
	}

	return u, nil
}

// HashPassword is exposed for trusted callers such as the seed command.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func (s *Service) issue(u *domain.User) (*Session, error) {
	accessToken, expiresAt, err := s.tokens.Issue(u.ID)
	if err != nil {
		return nil, err
	}

	s.log.Info("user authenticated", "user_id", u.ID)
	return &Session{User: u, AccessToken: accessToken, ExpiresAt: expiresAt}, nil
}

func invalidCredentials() error {
	return fmt.Errorf("%w: invalid email or password", domain.ErrUnauthorized)
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("%w: password must be at least %d characters", domain.ErrInvalid, minPasswordLen)
	}
	if len(password) > maxPasswordBytes {
		return fmt.Errorf("%w: password must be at most %d bytes", domain.ErrInvalid, maxPasswordBytes)
	}
	return nil
}
