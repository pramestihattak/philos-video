package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"philos-video/internal/models"
	sessionrepo "philos-video/internal/storage/session"
	videorepo "philos-video/internal/storage/video"
)

// PlaybackClaims is the JWT claims structure for playback tokens.
type PlaybackClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
	VideoID   string `json:"vid,omitempty"`  // set for VOD sessions
	StreamID  string `json:"stid,omitempty"` // set for live sessions
}

// Service manages playback sessions and issues JWT tokens.
type Service struct {
	sessions  sessionrepo.Repository
	videos    videorepo.Repository
	jwtSecret string
	jwtExpiry time.Duration
}

// New creates a session Service. Returns an error if jwtExpiry is not a valid duration.
func New(
	sessions sessionrepo.Repository,
	videos videorepo.Repository,
	jwtSecret, jwtExpiry string,
) (*Service, error) {
	d, err := time.ParseDuration(jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRY %q: %w", jwtExpiry, err)
	}
	return &Service{
		sessions:  sessions,
		videos:    videos,
		jwtSecret: jwtSecret,
		jwtExpiry: d,
	}, nil
}

func (s *Service) generateToken(sess *models.PlaybackSession) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.jwtExpiry)
	claims := PlaybackClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        sess.ID,
		},
		SessionID: sess.ID,
		VideoID:   sess.VideoID,
		StreamID:  sess.StreamID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtSecret))
	return signed, expiresAt, err
}

func genSessionID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sess_" + hex.EncodeToString(b), nil
}
