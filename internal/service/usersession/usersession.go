package usersession

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// Service issues and verifies user-identity JWTs for the browser session cookie.
// Completely separate from PlaybackClaims.
type Service struct {
	secret []byte
	secure bool // whether to set Secure flag on the cookie
}

// New creates a user session Service.
func New(secret string, secure bool) (*Service, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("session cookie secret must be at least 32 chars")
	}
	return &Service{secret: []byte(secret), secure: secure}, nil
}
