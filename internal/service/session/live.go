package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"philos-video/internal/models"
)

// CreateLiveSession creates a JWT-backed viewer session for a live stream.
func (s *Service) CreateLiveSession(ctx context.Context, streamID, deviceType, userAgent, ipAddress string) (*models.PlaybackSession, string, time.Time, error) {
	id, err := genSessionID()
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("generating session ID: %w", err)
	}

	sess := &models.PlaybackSession{
		ID:         id,
		StreamID:   streamID,
		DeviceType: deviceType,
		UserAgent:  userAgent,
		IPAddress:  ipAddress,
		Status:     "active",
	}

	tokenStr, expiresAt, err := s.generateToken(sess)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("generating token: %w", err)
	}
	sess.Token = tokenStr

	if err := s.sessions.Create(ctx, sess); err != nil {
		return nil, "", time.Time{}, fmt.Errorf("saving session: %w", err)
	}

	slog.Info("live session created",
		slog.String("session_id", sess.ID),
		slog.String("stream_id", streamID),
	)

	return sess, tokenStr, expiresAt, nil
}
