// Package admin implements the administrative use cases. It is deliberately a
// separate module from the user/auth services: only routes under
// /api/v1/admin reach it, and the transport layer requires an admin identity.
package admin

import (
	"context"
	"fmt"
	"log/slog"

	"api-hamy/internal/domain"
	userservice "api-hamy/internal/service/user"
)

// Service implements user administration.
type Service struct {
	users *userservice.Service
	log   *slog.Logger
}

func New(users *userservice.Service, log *slog.Logger) *Service {
	return &Service{users: users, log: log}
}

// List returns every active user.
func (s *Service) List(ctx context.Context, f domain.ListFilter) ([]domain.User, int64, error) {
	return s.users.List(ctx, f)
}

// Get returns an active user.
func (s *Service) Get(ctx context.Context, id string) (*domain.User, error) {
	return s.users.Get(ctx, id)
}

// Update changes a user's profile or admin flag. An admin cannot revoke their
// own flag, which would leave the instance without an administrator.
func (s *Service) Update(ctx context.Context, actorID, targetID string, in userservice.UpdateInput) (*domain.User, error) {
	if actorID == targetID && in.IsAdmin != nil && !*in.IsAdmin {
		return nil, fmt.Errorf("%w: you cannot revoke your own admin role", domain.ErrInvalid)
	}

	u, err := s.users.Update(ctx, targetID, in)
	if err != nil {
		return nil, err
	}

	s.log.Info("admin updated user", "actor_id", actorID, "user_id", targetID)
	return u, nil
}

// Delete soft-deletes a user. Admins cannot delete their own account here.
func (s *Service) Delete(ctx context.Context, actorID, targetID string) error {
	if actorID == targetID {
		return fmt.Errorf("%w: you cannot delete your own account", domain.ErrInvalid)
	}

	if err := s.users.Delete(ctx, targetID); err != nil {
		return err
	}

	s.log.Info("admin deleted user", "actor_id", actorID, "user_id", targetID)
	return nil
}
