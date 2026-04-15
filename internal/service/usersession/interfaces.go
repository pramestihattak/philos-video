package usersession

import (
	"net/http"

	"philos-video/internal/models"
)

// Servicer is the interface for browser session cookie operations.
type Servicer interface {
	Issue(user *models.User) (string, error)
	Parse(tokenStr string) (*UserClaims, error)
	SetSessionCookie(w http.ResponseWriter, user *models.User) error
	ClearSessionCookie(w http.ResponseWriter)
}
