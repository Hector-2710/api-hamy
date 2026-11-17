package domain

import "errors"

// Domain errors. The HTTP boundary is the only place that maps them to status codes.
var (
	ErrNotFound  = errors.New("recurso no encontrado")
	ErrConflict  = errors.New("el recurso ya existe")
	ErrInvalid   = errors.New("datos inválidos")
	ErrForbidden = errors.New("no tienes permiso para esta acción")
)
