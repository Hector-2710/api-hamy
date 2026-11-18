package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authservice "api-hamy/internal/service/auth"
	"api-hamy/internal/transport/http/dto"
	"api-hamy/internal/transport/http/middleware"
	"api-hamy/internal/transport/http/response"
)

// AuthHandler exposes registration, login and the current-user endpoint.
type AuthHandler struct {
	svc *authservice.Service
}

func NewAuthHandler(svc *authservice.Service) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Register creates an account and returns a session with an access token.
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	session, err := h.svc.Register(c.Request.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, dto.NewSessionResponse(session))
}

// Login verifies credentials and returns a session with an access token.
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	session, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.JSON(c, http.StatusOK, dto.NewSessionResponse(session))
}

// Me returns the authenticated user.
func (h *AuthHandler) Me(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.Unauthorized(c, "missing identity")
		return
	}

	response.JSON(c, http.StatusOK, u)
}
