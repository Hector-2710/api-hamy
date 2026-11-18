package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS allows the configured origins to call the API from a browser.
//
// A single "*" disables the origin allowlist. Requests without an Origin
// header (curl, server-to-server) pass through untouched.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	wildcard := false
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			wildcard = true
			continue
		}
		allowed[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		switch {
		case wildcard:
			c.Header("Access-Control-Allow-Origin", "*")
		default:
			if _, ok := allowed[origin]; !ok {
				// Not allowed: do not echo CORS headers. The browser blocks
				// the response, while the API still answers normally.
				c.Next()
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Max-Age", "600")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
