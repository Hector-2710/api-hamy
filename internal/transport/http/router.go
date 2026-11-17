package http

import (
	"github.com/gin-gonic/gin"

	"api-hamy/internal/transport/http/handler"
	"api-hamy/internal/transport/http/middleware"
)

// NewRouter registers all API routes.
func NewRouter(userHandler *handler.UserHandler, albumHandler *handler.AlbumHandler, photoHandler *handler.PhotoHandler) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	{
		// Open user signup (bootstrap). Everything else requires identity.
		users := v1.Group("/users")
		users.POST("", userHandler.Create)
		users.GET("", userHandler.List)
		users.GET("/:id", userHandler.Get)
		users.PATCH("/:id", userHandler.Update)
		users.DELETE("/:id", userHandler.Delete)
	}

	// TODO: replace middleware.Actor with JWT auth once it exists.
	secure := v1.Group("", middleware.Actor())
	{
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

	return r
}
