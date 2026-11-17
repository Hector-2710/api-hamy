package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"api-hamy/internal/domain"
	photoservice "api-hamy/internal/service/photo"
	"api-hamy/internal/transport/http/dto"
	"api-hamy/internal/transport/http/middleware"
	"api-hamy/internal/transport/http/response"
)

// PhotoHandler translates HTTP requests into service calls for the photo flow.
type PhotoHandler struct {
	svc *photoservice.Service
}

func NewPhotoHandler(svc *photoservice.Service) *PhotoHandler {
	return &PhotoHandler{svc: svc}
}

func (h *PhotoHandler) InitUpload(c *gin.Context) {
	var req dto.InitUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ticket, err := h.svc.InitUpload(c.Request.Context(), middleware.ActorID(c), c.Param("id"), req.Filename, req.ContentType, req.SizeBytes)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, dto.UploadTicketResponse{
		PhotoID:   ticket.PhotoID,
		UploadURL: ticket.UploadURL,
		ExpiresAt: ticket.ExpiresAt,
	})
}

func (h *PhotoHandler) CompleteUpload(c *gin.Context) {
	var req dto.CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	p, err := h.svc.Complete(c.Request.Context(), middleware.ActorID(c), c.Param("id"), c.Param("photoId"), req.Width, req.Height)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.NewPhotoResponse(p))
}

func (h *PhotoHandler) List(c *gin.Context) {
	var q dto.ListPhotosQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	filter := domain.ListFilter{Page: q.Page, PerPage: q.PerPage, Search: q.Search}.Normalize()

	photos, total, err := h.svc.List(c.Request.Context(), middleware.ActorID(c), c.Param("id"), filter)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"items": dto.NewPhotoResponses(photos),
		"meta": gin.H{
			"page":     filter.Page,
			"per_page": filter.PerPage,
			"total":    total,
		},
	})
}

func (h *PhotoHandler) DownloadURL(c *gin.Context) {
	url, expiresAt, err := h.svc.DownloadURL(c.Request.Context(), middleware.ActorID(c), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.DownloadURLResponse{URL: url, ExpiresAt: expiresAt})
}

func (h *PhotoHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), middleware.ActorID(c), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
