package common

import (
	"errors"
	"net/http"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrUnavailable     = errors.New("service unavailable")
)

// HTTPError maps wrapped domain errors without exposing their internal details.
func HTTPError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrInvalidArgument):
		return http.StatusBadRequest, "Invalid request"
	case errors.Is(err, ErrUnauthenticated):
		return http.StatusUnauthorized, "Authentication required"
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden, "Permission denied"
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound, "Resource not found"
	case errors.Is(err, ErrConflict):
		return http.StatusConflict, "Resource has changed or operation conflicts with its state"
	case errors.Is(err, ErrUnavailable):
		return http.StatusServiceUnavailable, "Service temporarily unavailable"
	default:
		return http.StatusInternalServerError, "Internal server error"
	}
}
