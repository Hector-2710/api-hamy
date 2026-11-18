package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DBPinger is the database dependency of the readiness check.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// StorageProbe verifies that the object storage is reachable.
type StorageProbe interface {
	EnsureBucket(ctx context.Context) error
}

// HealthHandler serves the readiness probe. Liveness (/healthz) is intentionally
// dependency-free and lives in the router.
type HealthHandler struct {
	db      DBPinger
	storage StorageProbe
}

func NewHealthHandler(db DBPinger, storage StorageProbe) *HealthHandler {
	return &HealthHandler{db: db, storage: storage}
}

// Ready reports whether the API can serve requests: Postgres and MinIO must
// respond within two seconds.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	checks := gin.H{
		"database": "ok",
		"storage":  "ok",
	}
	status := http.StatusOK
	overall := "ok"

	if err := h.db.Ping(ctx); err != nil {
		slog.Error("readiness: database is not reachable", "err", err)
		checks["database"] = "error"
		status = http.StatusServiceUnavailable
		overall = "unavailable"
	}

	if err := h.storage.EnsureBucket(ctx); err != nil {
		slog.Error("readiness: object storage is not reachable", "err", err)
		checks["storage"] = "error"
		status = http.StatusServiceUnavailable
		overall = "unavailable"
	}

	c.JSON(status, gin.H{
		"status": overall,
		"checks": checks,
	})
}
