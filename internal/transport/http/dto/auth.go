package dto

import (
	"time"

	"api-hamy/internal/domain"
	authservice "api-hamy/internal/service/auth"
)

// RegisterRequest is the body of POST /api/v1/auth/register.
type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// LoginRequest is the body of POST /api/v1/auth/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// SessionResponse is the payload returned after register/login.
type SessionResponse struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	ExpiresAt   time.Time   `json:"expires_at"`
	User        domain.User `json:"user"`
}

// NewSessionResponse maps a service session to the API contract.
func NewSessionResponse(s *authservice.Session) SessionResponse {
	return SessionResponse{
		AccessToken: s.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   s.ExpiresAt,
		User:        *s.User,
	}
}
