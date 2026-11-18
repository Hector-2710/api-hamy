package http

import (
	"github.com/gin-gonic/gin"

	"api-hamy/internal/transport/http/handler"
	"api-hamy/internal/transport/http/middleware"
)

// NewRouter registers all API routes.
func NewRouter(
	healthHandler *handler.HealthHandler,
	authHandler *handler.AuthHandler,
	authenticator middleware.Authenticator,
	albumHandler *handler.AlbumHandler,
	photoHandler *handler.PhotoHandler,
	adminHandler *handler.AdminHandler,
	allowedOrigins []string,
) *gin.Engine {
	r := gin.Default()
	r.Use(middleware.CORS(allowedOrigins))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/readyz", healthHandler.Ready)

	v1 := r.Group("/api/v1")
	{
		// Public: only the authentication endpoints are open.
		auth := v1.Group("/auth")
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
	}

	secure := v1.Group("", middleware.Auth(authenticator))
	{
		secure.GET("/auth/me", authHandler.Me)

		albums := secure.Group("/albums")
		albums.POST("", albumHandler.Create)
		albums.GET("", albumHandler.List)
		albums.GET("/:id", albumHandler.Get)
		albums.PATCH("/:id", albumHandler.Update)
		albums.DELETE("/:id", albumHandler.Delete)

		albums.PUT("/:id/cover", albumHandler.SetCover)
		albums.DELETE("/:id/cover", albumHandler.ClearCover)

		albums.POST("/:id/members", albumHandler.Invite)
		albums.GET("/:id/members", albumHandler.ListMembers)
		albums.PATCH("/:id/members/:userId", albumHandler.UpdateMember)
		albums.DELETE("/:id/members/:userId", albumHandler.RemoveMember)

		albums.GET("/:id/photos", photoHandler.List)
		albums.POST("/:id/photos/uploads", photoHandler.InitUpload)
		albums.POST("/:id/photos/:photoId/complete", photoHandler.CompleteUpload)

		secure.GET("/invitations", albumHandler.ListInvitations)
		secure.POST("/invitations/:albumId/accept", albumHandler.AcceptInvitation)

		secure.GET("/photos/:id/download-url", photoHandler.DownloadURL)
		secure.DELETE("/photos/:id", photoHandler.Delete)
	}

	// Separate administration module. Every route requires an admin identity.
	admin := secure.Group("/admin", middleware.RequireAdmin())
	{
		admin.GET("/users", adminHandler.ListUsers)
		admin.GET("/users/:id", adminHandler.GetUser)
		admin.PATCH("/users/:id", adminHandler.UpdateUser)
		admin.DELETE("/users/:id", adminHandler.DeleteUser)
	}

	return r
}
