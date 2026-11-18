package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"api-hamy/internal/domain"
	albumservice "api-hamy/internal/service/album"
	"api-hamy/internal/transport/http/dto"
	"api-hamy/internal/transport/http/middleware"
	"api-hamy/internal/transport/http/response"
)

// AlbumHandler translates HTTP requests into service calls and responds with JSON.
type AlbumHandler struct {
	svc *albumservice.Service
}

func NewAlbumHandler(svc *albumservice.Service) *AlbumHandler {
	return &AlbumHandler{svc: svc}
}

func (h *AlbumHandler) Create(c *gin.Context) {
	var req dto.CreateAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	a, err := h.svc.Create(c.Request.Context(), middleware.ActorID(c), req.Title, req.Description)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, dto.NewAlbumResponse(a))
}

func (h *AlbumHandler) List(c *gin.Context) {
	var q dto.ListAlbumsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	filter := domain.ListFilter{Page: q.Page, PerPage: q.PerPage, Search: q.Search}.Normalize()

	albums, total, err := h.svc.List(c.Request.Context(), middleware.ActorID(c), filter)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"items": dto.NewAlbumResponses(albums),
		"meta": gin.H{
			"page":     filter.Page,
			"per_page": filter.PerPage,
			"total":    total,
		},
	})
}

func (h *AlbumHandler) Get(c *gin.Context) {
	a, err := h.svc.Get(c.Request.Context(), middleware.ActorID(c), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.NewAlbumResponse(a))
}

func (h *AlbumHandler) Update(c *gin.Context) {
	var req dto.UpdateAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	a, err := h.svc.Update(c.Request.Context(), middleware.ActorID(c), c.Param("id"), req.Title, req.Description)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.NewAlbumResponse(a))
}

func (h *AlbumHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), middleware.ActorID(c), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AlbumHandler) SetCover(c *gin.Context) {
	var req dto.SetCoverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	a, err := h.svc.SetCover(c.Request.Context(), middleware.ActorID(c), c.Param("id"), &req.PhotoID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.NewAlbumResponse(a))
}

func (h *AlbumHandler) ClearCover(c *gin.Context) {
	a, err := h.svc.SetCover(c.Request.Context(), middleware.ActorID(c), c.Param("id"), nil)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.NewAlbumResponse(a))
}

func (h *AlbumHandler) Invite(c *gin.Context) {
	var req dto.InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if (req.UserID == "") == (req.Email == "") {
		response.BadRequest(c, "provide exactly one of: user_id or email")
		return
	}

	member, err := h.svc.Invite(c.Request.Context(), middleware.ActorID(c), c.Param("id"), req.UserID, req.Email, domain.MemberRole(req.Role))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, dto.NewMemberResponse(member))
}

func (h *AlbumHandler) ListMembers(c *gin.Context) {
	members, err := h.svc.ListMembers(c.Request.Context(), middleware.ActorID(c), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.NewMemberResponses(members))
}

func (h *AlbumHandler) UpdateMember(c *gin.Context) {
	var req dto.UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.svc.ChangeRole(c.Request.Context(), middleware.ActorID(c), c.Param("id"), c.Param("userId"), domain.MemberRole(req.Role)); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AlbumHandler) RemoveMember(c *gin.Context) {
	if err := h.svc.RemoveMember(c.Request.Context(), middleware.ActorID(c), c.Param("id"), c.Param("userId")); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AlbumHandler) ListInvitations(c *gin.Context) {
	invites, err := h.svc.ListInvitations(c.Request.Context(), middleware.ActorID(c))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.NewInvitationResponses(invites))
}

func (h *AlbumHandler) AcceptInvitation(c *gin.Context) {
	member, err := h.svc.Accept(c.Request.Context(), middleware.ActorID(c), c.Param("albumId"))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.NewMemberResponse(member))
}
