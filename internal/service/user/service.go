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

// Service implements the user use cases.
type Service struct {
	repo domain.UserRepository
	log  *slog.Logger
}

func New(repo domain.UserRepository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// Create normalizes the email, validates the input and persists the user.
func (s *Service) Create(ctx context.Context, name, email string) (*domain.User, error) {
	name = strings.TrimSpace(name)
	email = normalizeEmail(email)

	if err := validateName(name); err != nil {
		return nil, err
	}
	if err := validateEmail(email); err != nil {
		return nil, err
	}

	u := &domain.User{Name: name, Email: email}
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

// List returns paginated users and the total count.
func (s *Service) List(ctx context.Context, f domain.ListFilter) ([]domain.User, int64, error) {
	f = f.Normalize()
	f.Search = strings.TrimSpace(f.Search)

	return s.repo.List(ctx, f)
}

// Update applies a partial PATCH: nil means the field is unchanged.
func (s *Service) Update(ctx context.Context, id string, name, email *string) (*domain.User, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if err := validateName(trimmed); err != nil {
			return nil, err
		}
		current.Name = trimmed
	}

	if email != nil {
		normalized := normalizeEmail(*email)
		if err := validateEmail(normalized); err != nil {
			return nil, err
		}
		current.Email = normalized
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateName(name string) error {
	n := len([]rune(name))
	if n < minNameLen || n > maxNameLen {
		return fmt.Errorf("%w: name must be between %d and %d characters", domain.ErrInvalid, minNameLen, maxNameLen)
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" || len(email) > maxEmailLen {
		return fmt.Errorf("%w: invalid email", domain.ErrInvalid)
	}

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return fmt.Errorf("%w: invalid email", domain.ErrInvalid)
	}

	return nil
}
