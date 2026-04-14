package service

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors returned by service methods. Handlers map these to HTTP status codes.
var (
	// ErrNotFound is returned when a requested resource does not exist or is not
	// visible to the caller (to avoid information leaks).
	ErrNotFound = errors.New("not found")

	// ErrForbidden is returned when the caller lacks permission to perform an action.
	ErrForbidden = errors.New("forbidden")
)

// ValidationError wraps user-facing validation failures.
// Handlers that receive this type may return its message as a 400 response;
// all other errors should be treated as internal and return a generic 500.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

func validationErrorf(format string, args ...any) *ValidationError {
	return &ValidationError{msg: fmt.Sprintf(format, args...)}
}

// ErrQuotaExceeded is returned when a user's upload quota would be exceeded.
var ErrQuotaExceeded = &quotaError{}

type quotaError struct{}

func (e *quotaError) Error() string    { return "upload quota exceeded" }
func (e *quotaError) HTTPStatus() int  { return http.StatusRequestEntityTooLarge }
