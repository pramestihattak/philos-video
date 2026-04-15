package usersession

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"philos-video/internal/models"
)

// Issue creates a signed JWT for the given user.
func (s *Service) Issue(user *models.User) (string, error) {
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
