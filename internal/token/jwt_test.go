package token

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-with-at-least-32-characters"

func TestIssueAndParse(t *testing.T) {
	m, err := NewManager(testSecret, "test-issuer", time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	raw, expiresAt, err := m.Issue("user-123")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if raw == "" || expiresAt.Before(time.Now().Add(30*time.Minute)) {
		t.Fatalf("Issue() = %q, %v; want a token valid for ~1h", raw, expiresAt)
	}

	subject, err := m.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if subject != "user-123" {
		t.Fatalf("Parse() subject = %q, want user-123", subject)
	}
}

func TestParseRejectsInvalidTokens(t *testing.T) {
	m, err := NewManager(testSecret, "test-issuer", time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	other, err := NewManager("another-secret-with-32-characters-ok", "test-issuer", time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	foreignToken, _, err := other.Issue("user-123")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	expired := sign(t, testSecret, Claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "user-123",
		Issuer:    "test-issuer",
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}})

	wrongIssuer := sign(t, testSecret, Claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "user-123",
		Issuer:    "someone-else",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}})

	noExpiration := sign(t, testSecret, Claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:  "user-123",
		Issuer:   "test-issuer",
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}})

	cases := map[string]string{
		"malformed":    "not-a-jwt",
		"wrong secret": foreignToken,
		"expired":      expired,
		"wrong issuer": wrongIssuer,
		"no expiry":    noExpiration,
	}

	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := m.Parse(raw); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("Parse() error = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestNewManagerValidation(t *testing.T) {
	cases := []struct {
		name   string
		secret string
		issuer string
		ttl    time.Duration
	}{
		{"short secret", "too-short", "issuer", time.Hour},
		{"empty issuer", testSecret, "", time.Hour},
		{"zero ttl", testSecret, "issuer", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewManager(tc.secret, tc.issuer, tc.ttl); err == nil {
				t.Fatal("NewManager() error = nil, want validation error")
			}
		})
	}
}

func sign(t *testing.T, secret string, claims Claims) string {
	t.Helper()

	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return raw
}
