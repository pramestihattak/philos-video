package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"philos-video/internal/models"
)

func (s *Service) CreateSession(ctx context.Context, videoID, deviceType, userAgent, ipAddress string) (*models.PlaybackSession, string, time.Time, error) {
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

	sess := &models.PlaybackSession{
		ID:         id,
		VideoID:    videoID,
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

	slog.Info("session created",
		slog.String("session_id", sess.ID),
		slog.String("video_id", videoID),
		slog.String("device_type", deviceType),
		slog.String("ip", ipAddress),
	)

	return sess, tokenStr, expiresAt, nil
}
