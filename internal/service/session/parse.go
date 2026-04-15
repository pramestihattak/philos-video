package session

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

func (s *Service) ParseToken(tokenStr string) (*PlaybackClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &PlaybackClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*PlaybackClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
