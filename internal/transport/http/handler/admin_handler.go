package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"api-hamy/internal/domain"
	adminservice "api-hamy/internal/service/admin"
	userservice "api-hamy/internal/service/user"
	"api-hamy/internal/transport/http/dto"
	"api-hamy/internal/transport/http/middleware"
	"api-hamy/internal/transport/http/response"
)

// AdminHandler exposes the administrative API. It is only mounted behind the
// JWT + RequireAdmin middleware chain.
type AdminHandler struct {
	svc *adminservice.Service
}

func NewAdminHandler(svc *adminservice.Service) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	var q dto.ListUsersQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	filter := domain.ListFilter{Page: q.Page, PerPage: q.PerPage, Search: q.Search}.Normalize()

	users, total, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"items": users,
		"meta": gin.H{
			"page":     filter.Page,
			"per_page": filter.PerPage,
			"total":    total,
		},
	})
}

func (h *AdminHandler) GetUser(c *gin.Context) {
	u, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, u)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	var req dto.UpdateAdminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	u, err := h.svc.Update(c.Request.Context(), middleware.ActorID(c), c.Param("id"), userservice.UpdateInput{
		Name:    req.Name,
		Email:   req.Email,
		IsAdmin: req.IsAdmin,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, u)
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), middleware.ActorID(c), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
