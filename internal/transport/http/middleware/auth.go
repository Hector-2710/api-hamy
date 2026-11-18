// Package middleware contains cross-cutting HTTP middleware.
package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"api-hamy/internal/domain"
	"api-hamy/internal/transport/http/response"
)

const (
	actorContextKey = "actor_id"
	userContextKey  = "actor_user"
)

// Authenticator resolves a bearer token into the current user.
type Authenticator interface {
	Authenticate(ctx context.Context, rawToken string) (*domain.User, error)
}

// Auth requires a valid JWT in the Authorization header and stores the user in
// the request context. The user is reloaded from the database on every request.
func Auth(authenticator Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			response.Unauthorized(c, "missing or malformed Authorization header")
			c.Abort()
			return
		}

		u, err := authenticator.Authenticate(c.Request.Context(), raw)
		if err != nil {
			response.Error(c, err)
			c.Abort()
			return
		}

		c.Set(actorContextKey, u.ID)
		c.Set(userContextKey, u)
		c.Next()
	}
}

// RequireAdmin rejects authenticated users without the admin flag.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil || !u.IsAdmin {
			response.Forbidden(c, "admin access required")
			c.Abort()
			return
		}

		c.Next()
	}
}

// ActorID returns the authenticated user's ID.
func ActorID(c *gin.Context) string {
	value, ok := c.Get(actorContextKey)
	if !ok {
		return ""
	}
	id, _ := value.(string)
	return id
}

// CurrentUser returns the authenticated user or nil.
func CurrentUser(c *gin.Context) *domain.User {
	value, ok := c.Get(userContextKey)
	if !ok {
		return nil
	}
	u, _ := value.(*domain.User)
	return u
}

func bearerToken(header string) (string, bool) {
	scheme, raw, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	return raw, true
}
