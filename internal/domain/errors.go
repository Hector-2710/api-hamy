package domain

import "errors"

// Domain errors. The HTTP boundary is the only place that maps them to status codes.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists")
	ErrInvalid      = errors.New("invalid data")
	ErrForbidden    = errors.New("you do not have permission for this action")
	ErrUnauthorized = errors.New("invalid credentials or token")
)
