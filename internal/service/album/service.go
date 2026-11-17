package album

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"api-hamy/internal/domain"
)

const (
	maxTitleLen       = 150
	maxDescriptionLen = 2000
)

// Service implements the album and membership use cases.
type Service struct {
	albums domain.AlbumRepository
	users  domain.UserRepository
	log    *slog.Logger
}

func New(albums domain.AlbumRepository, users domain.UserRepository, log *slog.Logger) *Service {
	return &Service{albums: albums, users: users, log: log}
}

// Create creates an album owned by the actor.
func (s *Service) Create(ctx context.Context, actorID, title string, description *string) (*domain.Album, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxTitleLen {
		return nil, fmt.Errorf("%w: title must be between 1 and %d characters", domain.ErrInvalid, maxTitleLen)
	}

	desc, err := normalizeDescription(description)
	if err != nil {
		return nil, err
	}

	a := &domain.Album{OwnerID: actorID, Title: title, Description: desc}
	if err := s.albums.Create(ctx, a); err != nil {
		return nil, err
	}

	s.log.Info("album created", "album_id", a.ID, "owner_id", actorID)
	return a, nil
}

// Get returns an album if the actor is the owner or an active member.
func (s *Service) Get(ctx context.Context, actorID, albumID string) (*domain.Album, error) {
	return s.albums.GetForActor(ctx, albumID, actorID)
}

// List returns the albums owned by and shared with the actor.
func (s *Service) List(ctx context.Context, actorID string, f domain.ListFilter) ([]domain.Album, int64, error) {
	f = f.Normalize()
	f.Search = strings.TrimSpace(f.Search)

	return s.albums.ListForActor(ctx, actorID, f)
}

// Update applies a partial PATCH. Only the owner can edit the album.
func (s *Service) Update(ctx context.Context, actorID, albumID string, title, description *string) (*domain.Album, error) {
	if _, err := s.requireOwner(ctx, actorID, albumID); err != nil {
		return nil, err
	}

	var clearDescription bool
	if title != nil {
		trimmed := strings.TrimSpace(*title)
		if trimmed == "" || utf8.RuneCountInString(trimmed) > maxTitleLen {
			return nil, fmt.Errorf("%w: title must be between 1 and %d characters", domain.ErrInvalid, maxTitleLen)
		}
		title = &trimmed
	}

	if description != nil {
		desc, err := normalizeDescription(description)
		if err != nil {
			return nil, err
		}
		if desc == nil {
			clearDescription = true
		}
		description = desc
	}

	return s.albums.Update(ctx, albumID, actorID, title, description, clearDescription)
}

// SetCover sets or clears (photoID nil) the album cover. Only the owner.
func (s *Service) SetCover(ctx context.Context, actorID, albumID string, photoID *string) (*domain.Album, error) {
	if _, err := s.requireOwner(ctx, actorID, albumID); err != nil {
		return nil, err
	}

	a, err := s.albums.SetCover(ctx, albumID, actorID, photoID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Ownership was already checked, so the only remaining cause is a
			// missing, deleted, unconfirmed or foreign photo.
			return nil, fmt.Errorf("%w: photo does not belong to the album or is not ready", domain.ErrInvalid)
		}
		return nil, err
	}

	return a, nil
}

// Delete soft-deletes the album. Only the owner.
func (s *Service) Delete(ctx context.Context, actorID, albumID string) error {
	if _, err := s.requireOwner(ctx, actorID, albumID); err != nil {
		return err
	}

	if err := s.albums.Delete(ctx, albumID, actorID); err != nil {
		return err
	}

	s.log.Info("album deleted", "album_id", albumID, "actor_id", actorID)
	return nil
}

// Invite invites a user by ID or email with a viewer/editor role.
// If the user is already an active member, only the role is updated.
func (s *Service) Invite(ctx context.Context, actorID, albumID, userID, email string, role domain.MemberRole) (*domain.AlbumMember, error) {
	a, err := s.requireOwner(ctx, actorID, albumID)
	if err != nil {
		return nil, err
	}

	if role == "" {
		role = domain.RoleViewer
	}
	if !role.IsValidRole() {
		return nil, fmt.Errorf("%w: invalid role (viewer or editor)", domain.ErrInvalid)
	}

	target, err := s.resolveUser(ctx, userID, email)
	if err != nil {
		return nil, err
	}

	if target.ID == a.OwnerID {
		return nil, fmt.Errorf("%w: the owner already has access to the album", domain.ErrConflict)
	}
	if target.ID == actorID {
		return nil, fmt.Errorf("%w: you cannot invite yourself", domain.ErrInvalid)
	}

	existing, err := s.albums.GetMember(ctx, albumID, target.ID)
	switch {
	case err == nil && existing.Status == domain.StatusActive:
		if err := s.albums.UpdateMemberRole(ctx, albumID, target.ID, role); err != nil {
			return nil, err
		}
		existing.Role = role
		return existing, nil
	case err != nil && !errors.Is(err, domain.ErrNotFound):
		return nil, err
	}

	member, err := s.albums.UpsertMember(ctx, &domain.AlbumMember{
		AlbumID:   albumID,
		UserID:    target.ID,
		Role:      role,
		Status:    domain.StatusInvited,
		InvitedBy: &actorID,
	})
	if err != nil {
		return nil, err
	}

	s.log.Info("invitation sent", "album_id", albumID, "user_id", target.ID, "role", role)
	return member, nil
}

// Accept activates the actor's pending invitation. It is idempotent.
func (s *Service) Accept(ctx context.Context, actorID, albumID string) (*domain.AlbumMember, error) {
	if err := s.albums.ActivateMember(ctx, albumID, actorID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Already an active member: return the current state instead of failing.
			if member, getErr := s.albums.GetMember(ctx, albumID, actorID); getErr == nil && member.Status == domain.StatusActive {
				return member, nil
			}
		}
		return nil, err
	}

	member, err := s.albums.GetMember(ctx, albumID, actorID)
	if err != nil {
		return nil, err
	}

	s.log.Info("invitation accepted", "album_id", albumID, "user_id", actorID)
	return member, nil
}

// ListInvitations returns the actor's pending invitations.
func (s *Service) ListInvitations(ctx context.Context, actorID string) ([]domain.AlbumInvitation, error) {
	return s.albums.ListInvitations(ctx, actorID)
}

// ListMembers lists the active/invited members of an album for any member.
func (s *Service) ListMembers(ctx context.Context, actorID, albumID string) ([]domain.AlbumMember, error) {
	if _, err := s.albums.GetForActor(ctx, albumID, actorID); err != nil {
		return nil, err
	}

	return s.albums.ListMembers(ctx, albumID, actorID)
}

// ChangeRole changes a member's role. Only the owner.
func (s *Service) ChangeRole(ctx context.Context, actorID, albumID, targetUserID string, role domain.MemberRole) error {
	a, err := s.requireOwner(ctx, actorID, albumID)
	if err != nil {
		return err
	}

	if !role.IsValidRole() {
		return fmt.Errorf("%w: invalid role (viewer or editor)", domain.ErrInvalid)
	}
	if targetUserID == a.OwnerID {
		return fmt.Errorf("%w: cannot change the owner's role", domain.ErrInvalid)
	}

	return s.albums.UpdateMemberRole(ctx, albumID, targetUserID, role)
}

// RemoveMember removes a member (owner) or makes the actor leave the album.
func (s *Service) RemoveMember(ctx context.Context, actorID, albumID, targetUserID string) error {
	if targetUserID == actorID {
		a, err := s.albums.GetForActor(ctx, albumID, actorID)
		if err != nil {
			return err
		}
		if a.AccessRole.IsOwner() {
			return fmt.Errorf("%w: the owner cannot leave the album; delete it or transfer ownership", domain.ErrInvalid)
		}
		return s.albums.RevokeMember(ctx, albumID, actorID)
	}

	a, err := s.requireOwner(ctx, actorID, albumID)
	if err != nil {
		return err
	}
	if targetUserID == a.OwnerID {
		return fmt.Errorf("%w: cannot remove the owner", domain.ErrInvalid)
	}

	if err := s.albums.RevokeMember(ctx, albumID, targetUserID); err != nil {
		return err
	}

	s.log.Info("member removed", "album_id", albumID, "user_id", targetUserID, "actor_id", actorID)
	return nil
}

func (s *Service) requireOwner(ctx context.Context, actorID, albumID string) (*domain.Album, error) {
	a, err := s.albums.GetForActor(ctx, albumID, actorID)
	if err != nil {
		return nil, err
	}
	if !a.AccessRole.IsOwner() {
		return nil, domain.ErrForbidden
	}
	return a, nil
}

func (s *Service) resolveUser(ctx context.Context, userID, email string) (*domain.User, error) {
	switch {
	case strings.TrimSpace(userID) != "":
		return s.users.GetByID(ctx, strings.TrimSpace(userID))
	case strings.TrimSpace(email) != "":
		return s.users.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	default:
		return nil, fmt.Errorf("%w: provide user_id or email", domain.ErrInvalid)
	}
}

func normalizeDescription(description *string) (*string, error) {
	if description == nil {
		return nil, nil
	}

	trimmed := strings.TrimSpace(*description)
	if trimmed == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(trimmed) > maxDescriptionLen {
		return nil, fmt.Errorf("%w: description exceeds %d characters", domain.ErrInvalid, maxDescriptionLen)
	}

	return &trimmed, nil
}
