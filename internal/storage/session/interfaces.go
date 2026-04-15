package session

import (
	"context"

	"philos-video/internal/models"
)

// Repository defines the persistence operations for playback sessions.
type Repository interface {
	Create(ctx context.Context, s *models.PlaybackSession) error
	Get(ctx context.Context, id string) (*models.PlaybackSession, error)
	TouchLastActive(ctx context.Context, id string) error
	MarkEnded(ctx context.Context, id string) error
	CountActiveByStreamID(ctx context.Context, streamID string) (int, error)
}
