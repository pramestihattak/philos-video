package service

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"philos-video/internal/models"
)

const sessionCookieName = "philos_session"
const sessionDuration = 30 * 24 * time.Hour

// UserClaims is the JWT payload stored in the philos_session cookie.
// It is intentionally separate from PlaybackClaims (HLS URL tokens).
type UserClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"uid"`
	Email  string `json:"email"`
}

// UserSessionService issues and verifies user-identity JWTs for the
// browser session cookie. Completely separate from PlaybackClaims.
type UserSessionService struct {
	secret []byte
	secure bool // whether to set Secure flag on the cookie
}

func NewUserSessionService(secret string, secure bool) (*UserSessionService, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("session cookie secret must be at least 32 chars")
	}
	return &UserSessionService{secret: []byte(secret), secure: secure}, nil
}

// Issue creates a signed JWT for the given user.
func (s *UserSessionService) Issue(user *models.User) (string, error) {
	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(sessionDuration)),
		},
		UserID: user.ID,
		Email:  user.Email,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("signing session token: %w", err)
	}
	return signed, nil
}

// Parse validates a signed token string and returns its claims.
func (s *UserSessionService) Parse(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// SetSessionCookie writes a signed session JWT to the response as an HttpOnly cookie.
func (s *UserSessionService) SetSessionCookie(w http.ResponseWriter, user *models.User) error {
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
func (s *UserSessionService) ClearSessionCookie(w http.ResponseWriter) {
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
