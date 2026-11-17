// Package middleware contains cross-cutting HTTP middleware.
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"api-hamy/internal/transport/http/response"
)

const actorContextKey = "actor_id"

// Actor resolves the authenticated user.
//
// TODO: replace this stub with JWT validation once auth exists.
// Until then it uses the X-User-Id header (development and testing only).
func Actor() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader("X-User-Id"))
		if raw == "" {
			response.Unauthorized(c, "missing X-User-Id header")
			c.Abort()
			return
		}

		if _, err := uuid.Parse(raw); err != nil {
			response.BadRequest(c, "X-User-Id must be a UUID")
			c.Abort()
			return
		}

		c.Set(actorContextKey, raw)
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
