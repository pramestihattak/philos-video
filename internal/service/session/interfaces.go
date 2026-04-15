package session

import (
	"context"
	"time"

	"philos-video/internal/models"
)

// Servicer is the interface for playback session operations.
type Servicer interface {
	CreateSession(ctx context.Context, videoID, deviceType, userAgent, ipAddress string) (*models.PlaybackSession, string, time.Time, error)
	CreateLiveSession(ctx context.Context, streamID, deviceType, userAgent, ipAddress string) (*models.PlaybackSession, string, time.Time, error)
	ParseToken(tokenStr string) (*PlaybackClaims, error)
}
