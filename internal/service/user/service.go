package user

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"

	"api-hamy/internal/domain"
)

const (
	maxNameLen  = 100
	minNameLen  = 2
	maxEmailLen = 254
)

// CreateInput is the data needed to create a user.
//
// PasswordHash is optional at this layer: registration hashes the password
// before calling the service, while the seed may create users without one.
// IsAdmin is only honored by trusted callers (seed, admin bootstrap).
type CreateInput struct {
	Name         string
	Email        string
	PasswordHash string
	IsAdmin      bool
}

// UpdateInput is a partial update: nil means "leave unchanged".
type UpdateInput struct {
	Name         *string
	Email        *string
	PasswordHash *string
	IsAdmin      *bool
}

// Service implements the user use cases.
type Service struct {
	repo domain.UserRepository
	log  *slog.Logger
}

func New(repo domain.UserRepository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// Create normalizes the email, validates the input and persists the user.
func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.User, error) {
	name := strings.TrimSpace(in.Name)
	email := NormalizeEmail(in.Email)

	if err := ValidateName(name); err != nil {
		return nil, err
	}
	if err := ValidateEmail(email); err != nil {
		return nil, err
	}

	u := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: in.PasswordHash,
		IsAdmin:      in.IsAdmin,
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}

	s.log.Info("user created", "id", u.ID)
	return u, nil
}

// Get returns an active user by ID.
func (s *Service) Get(ctx context.Context, id string) (*domain.User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: empty id", domain.ErrInvalid)
	}

	return s.repo.GetByID(ctx, id)
}

// GetByEmail returns an active user by email.
func (s *Service) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.repo.GetByEmail(ctx, NormalizeEmail(email))
}

// List returns paginated users and the total count.
func (s *Service) List(ctx context.Context, f domain.ListFilter) ([]domain.User, int64, error) {
	f = f.Normalize()
	f.Search = strings.TrimSpace(f.Search)

	return s.repo.List(ctx, f)
}

// Update applies a partial PATCH: nil means the field is unchanged.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*domain.User, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if err := ValidateName(trimmed); err != nil {
			return nil, err
		}
		current.Name = trimmed
	}

	if in.Email != nil {
		normalized := NormalizeEmail(*in.Email)
		if err := ValidateEmail(normalized); err != nil {
			return nil, err
		}
		current.Email = normalized
	}

	if in.PasswordHash != nil && *in.PasswordHash != "" {
		current.PasswordHash = *in.PasswordHash
	}

	if in.IsAdmin != nil {
		current.IsAdmin = *in.IsAdmin
	}

	if err := s.repo.Update(ctx, current); err != nil {
		return nil, err
	}

	return current, nil
}

// Delete soft-deletes the user.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// NormalizeEmail lowercases and trims an email address.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateName checks the user's display name.
func ValidateName(name string) error {
	n := len([]rune(name))
	if n < minNameLen || n > maxNameLen {
		return fmt.Errorf("%w: name must be between %d and %d characters", domain.ErrInvalid, minNameLen, maxNameLen)
	}
	return nil
}

// ValidateEmail checks a normalized email address.
func ValidateEmail(email string) error {
	if email == "" || len(email) > maxEmailLen {
		return fmt.Errorf("%w: invalid email", domain.ErrInvalid)
	}

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return fmt.Errorf("%w: invalid email", domain.ErrInvalid)
	}

	return nil
}
