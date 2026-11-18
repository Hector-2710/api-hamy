package domain

import "time"

// User is the user domain entity.
//
// PasswordHash never leaves the domain: it is excluded from JSON and only the
// authentication service reads it.
type User struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	IsAdmin      bool       `json:"is_admin"`
	PasswordHash string     `json:"-"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
