package response

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"api-hamy/internal/domain"
)

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON responds with the {"data": ...} envelope.
func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

// BadRequest responds 400 for an invalid body or query.
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error: ErrorDetail{Code: "invalid_request", Message: message},
	})
}

// Unauthorized responds 401 when identity is missing.
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, ErrorResponse{
		Error: ErrorDetail{Code: "unauthorized", Message: message},
	})
}

// Error is the only place that maps domain errors to HTTP status codes.
func Error(c *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "internal error"

	switch {
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", err.Error()
	case errors.Is(err, domain.ErrConflict):
		status, code, message = http.StatusConflict, "conflict", err.Error()
	case errors.Is(err, domain.ErrInvalid):
		status, code, message = http.StatusUnprocessableEntity, "invalid", err.Error()
	case errors.Is(err, domain.ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", err.Error()
	default:
		// Unclassified errors become 500: log them fully and hide details from the client.
		slog.Error("unclassified internal error", "err", err, "path", c.FullPath(), "method", c.Request.Method)
	}

	c.JSON(status, ErrorResponse{Error: ErrorDetail{Code: code, Message: message}})
}
