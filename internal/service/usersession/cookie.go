package usersession

import (
	"net/http"

	"philos-video/internal/models"
)

// SetSessionCookie writes a signed session JWT to the response as an HttpOnly cookie.
func (s *Service) SetSessionCookie(w http.ResponseWriter, user *models.User) error {
	token, err := s.Issue(user)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// ClearSessionCookie removes the session cookie from the browser.
func (s *Service) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}
