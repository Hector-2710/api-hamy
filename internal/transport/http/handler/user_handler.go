package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"api-hamy/internal/domain"
	userservice "api-hamy/internal/service/user"
	"api-hamy/internal/transport/http/dto"
	"api-hamy/internal/transport/http/response"
)

// UserHandler translates HTTP requests into service calls and responds with JSON.
type UserHandler struct {
	svc *userservice.Service
}

func NewUserHandler(svc *userservice.Service) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) Create(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	u, err := h.svc.Create(c.Request.Context(), req.Name, req.Email)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, u)
}

func (h *UserHandler) Get(c *gin.Context) {
	u, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, u)
}

func (h *UserHandler) List(c *gin.Context) {
	var q dto.ListUsersQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	items, total, err := h.svc.List(c.Request.Context(), domain.ListFilter{
		Page:    q.Page,
		PerPage: q.PerPage,
		Search:  q.Search,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"items": items,
		"meta": gin.H{
			"page":     q.Page,
			"per_page": q.PerPage,
			"total":    total,
		},
	})
}

func (h *UserHandler) Update(c *gin.Context) {
	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	u, err := h.svc.Update(c.Request.Context(), c.Param("id"), req.Name, req.Email)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, u)
}

func (h *UserHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
