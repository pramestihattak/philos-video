package service

import "fmt"

// ValidationError wraps user-facing validation failures.
// Handlers that receive this type may return its message as a 400 response;
// all other errors should be treated as internal and return a generic 500.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

func validationErrorf(format string, args ...any) *ValidationError {
	return &ValidationError{msg: fmt.Sprintf(format, args...)}
}
