// Package token issues and validates the JWT access tokens.
package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned for malformed, expired or wrongly signed tokens.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims is the payload of an access token. The subject is the user ID; the
// user is always reloaded from the database on every request, so no
// authorization data is trusted from the token itself.
type Claims struct {
	jwt.RegisteredClaims
}

// Manager signs and verifies HS256 tokens.
type Manager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewManager validates the configuration and returns a token manager.
func NewManager(secret, issuer string, ttl time.Duration) (*Manager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT secret must be at least 32 bytes")
	}
	if issuer == "" {
		return nil, fmt.Errorf("JWT issuer must not be empty")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("JWT TTL must be positive")
	}

	return &Manager{secret: []byte(secret), issuer: issuer, ttl: ttl}, nil
}

// Issue returns a signed token and its expiration time.
func (m *Manager) Issue(userID string) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.ttl)

	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    m.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	return signed, expiresAt, nil
}

// Parse verifies the signature, issuer and expiration, and returns the user ID.
func (m *Manager) Parse(raw string) (string, error) {
	claims := &Claims{}

	parsed, err := jwt.ParseWithClaims(
		raw,
		claims,
		func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, ErrInvalidToken
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || !parsed.Valid || claims.Subject == "" {
		return "", ErrInvalidToken
	}

	return claims.Subject, nil
}
