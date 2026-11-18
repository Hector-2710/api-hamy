package dto

import (
	"time"

	"api-hamy/internal/domain"
)

// InviteMemberRequest is the body of POST /api/v1/albums/:id/members.
// Provide exactly one of user_id or email.
type InviteMemberRequest struct {
	UserID string `json:"user_id" binding:"omitempty,uuid"`
	Email  string `json:"email" binding:"omitempty,email"`
	Role   string `json:"role" binding:"omitempty,oneof=viewer editor"`
}

// UpdateMemberRequest is the body of PATCH /api/v1/albums/:id/members/:userId.
type UpdateMemberRequest struct {
	Role string `json:"role" binding:"required,oneof=viewer editor"`
}

// MemberResponse is the JSON representation of a membership.
type MemberResponse struct {
	AlbumID   string    `json:"album_id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name,omitempty"`
	Email     string    `json:"email,omitempty"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	InvitedBy *string   `json:"invited_by"`
	CreatedAt time.Time `json:"created_at"`
}

// NewMemberResponse maps a domain member to its JSON representation.
func NewMemberResponse(m *domain.AlbumMember) MemberResponse {
	return MemberResponse{
		AlbumID:   m.AlbumID,
		UserID:    m.UserID,
		Name:      m.UserName,
		Email:     m.UserEmail,
		Role:      string(m.Role),
		Status:    string(m.Status),
		InvitedBy: m.InvitedBy,
		CreatedAt: m.CreatedAt,
	}
}

// NewMemberResponses maps a list of domain members.
func NewMemberResponses(members []domain.AlbumMember) []MemberResponse {
	out := make([]MemberResponse, 0, len(members))
	for i := range members {
		out = append(out, NewMemberResponse(&members[i]))
	}
	return out
}

// InvitationResponse is a pending invitation as seen by the invitee.
type InvitationResponse struct {
	AlbumID        string    `json:"album_id"`
	AlbumTitle     string    `json:"album_title"`
	Role           string    `json:"role"`
	InvitedByName  *string   `json:"invited_by_name"`
	InvitedByEmail *string   `json:"invited_by_email"`
	CreatedAt      time.Time `json:"created_at"`
}

// NewInvitationResponse maps a domain invitation to its JSON representation.
func NewInvitationResponse(i *domain.AlbumInvitation) InvitationResponse {
	return InvitationResponse{
		AlbumID:        i.AlbumID,
		AlbumTitle:     i.AlbumTitle,
		Role:           string(i.Role),
		InvitedByName:  i.InvitedByName,
		InvitedByEmail: i.InvitedByEmail,
		CreatedAt:      i.CreatedAt,
	}
}

// NewInvitationResponses maps a list of domain invitations.
func NewInvitationResponses(invites []domain.AlbumInvitation) []InvitationResponse {
	out := make([]InvitationResponse, 0, len(invites))
	for i := range invites {
		out = append(out, NewInvitationResponse(&invites[i]))
	}
	return out
}
