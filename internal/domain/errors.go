package domain

import "errors"

// Domain errors. The service returns these; the handler layer maps them to HTTP
// status codes (404, 409, 422, ...). Nothing below the handler talks HTTP.
var (
	ErrNotFound      = errors.New("not found")                        // → 404
	ErrForbidden     = errors.New("forbidden")                        // → 403
	ErrInvalidStatus = errors.New("invalid status transition")        // → 422
	ErrConflict      = errors.New("concurrent modification conflict") // → 409 (optimistic lock)
)
