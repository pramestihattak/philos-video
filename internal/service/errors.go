package service

import (
	"errors"
	"fmt"
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
// Handlers check for this type via errors.As and return its message as a 400 response.
// All other errors should be treated as internal and return a generic 500.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

// NewValidationError creates a ValidationError with the given message.
func NewValidationError(msg string) *ValidationError {
	return &ValidationError{msg: msg}
}

// NewValidationErrorf creates a ValidationError with a formatted message.
func NewValidationErrorf(format string, args ...any) *ValidationError {
	return &ValidationError{msg: fmt.Sprintf(format, args...)}
}
