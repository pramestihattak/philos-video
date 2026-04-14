package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"philos-video/internal/models"
	"philos-video/internal/storage"
)

// PlaybackClaims is the JWT claims structure for playback tokens.
type PlaybackClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
	VideoID   string `json:"vid,omitempty"`  // set for VOD sessions
	StreamID  string `json:"stid,omitempty"` // set for live sessions
}

type SessionService struct {
	sessions  storage.SessionStorer
	videos    storage.VideoStorer
	jwtSecret string
	jwtExpiry time.Duration
}

func NewSessionService(
	sessions storage.SessionStorer,
	videos storage.VideoStorer,
	jwtSecret, jwtExpiry string,
) (*SessionService, error) {
	d, err := time.ParseDuration(jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRY %q: %w", jwtExpiry, err)
	}
	return &SessionService{
		sessions:  sessions,
		videos:    videos,
		jwtSecret: jwtSecret,
		jwtExpiry: d,
	}, nil
}

func (s *SessionService) CreateSession(ctx context.Context, videoID, deviceType, userAgent, ipAddress string) (*models.PlaybackSession, string, time.Time, error) {
	video, err := s.videos.GetByID(ctx, videoID)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("looking up video: %w", err)
	}
	if video == nil {
		return nil, "", time.Time{}, fmt.Errorf("video not found")
	}
	if video.Status != models.VideoStatusReady {
		return nil, "", time.Time{}, fmt.Errorf("video not ready (status: %s)", video.Status)
	}

	id, err := genSessionID()
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("generating session ID: %w", err)
	}

	session := &models.PlaybackSession{
		ID:         id,
		VideoID:    videoID,
		DeviceType: deviceType,
		UserAgent:  userAgent,
		IPAddress:  ipAddress,
		Status:     "active",
	}

	tokenStr, expiresAt, err := s.generateToken(session)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("generating token: %w", err)
	}
	session.Token = tokenStr

	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, "", time.Time{}, fmt.Errorf("saving session: %w", err)
	}

	slog.Info("session created",
		slog.String("session_id", session.ID),
		slog.String("video_id", videoID),
		slog.String("device_type", deviceType),
		slog.String("ip", ipAddress),
	)

	return session, tokenStr, expiresAt, nil
}

func (s *SessionService) ParseToken(tokenStr string) (*PlaybackClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &PlaybackClaims{}, func(t *jwt.Token) (interface{}, error) {
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

// CreateLiveSession creates a JWT-backed viewer session for a live stream.
func (s *SessionService) CreateLiveSession(ctx context.Context, streamID, deviceType, userAgent, ipAddress string) (*models.PlaybackSession, string, time.Time, error) {
	id, err := genSessionID()
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("generating session ID: %w", err)
	}

	session := &models.PlaybackSession{
		ID:         id,
		StreamID:   streamID,
		DeviceType: deviceType,
		UserAgent:  userAgent,
		IPAddress:  ipAddress,
		Status:     "active",
	}

	tokenStr, expiresAt, err := s.generateToken(session)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("generating token: %w", err)
	}
	session.Token = tokenStr

	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, "", time.Time{}, fmt.Errorf("saving session: %w", err)
	}

	slog.Info("live session created",
		slog.String("session_id", session.ID),
		slog.String("stream_id", streamID),
	)

	return session, tokenStr, expiresAt, nil
}

func (s *SessionService) generateToken(session *models.PlaybackSession) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.jwtExpiry)
	claims := PlaybackClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        session.ID,
		},
		SessionID: session.ID,
		VideoID:   session.VideoID,
		StreamID:  session.StreamID,
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
