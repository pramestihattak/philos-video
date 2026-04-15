package livestream

import (
	"context"

	"philos-video/internal/models"
)

// Repository defines the persistence operations for live streams.
type Repository interface {
	Create(ctx context.Context, streamKeyID, title string, recordVOD bool, userID string) (*models.LiveStream, error)
	GetByID(ctx context.Context, id string) (*models.LiveStream, error)
	GetByIDForUser(ctx context.Context, id, userID string) (*models.LiveStream, error)
	GetActiveByStreamKey(ctx context.Context, streamKeyID string) (*models.LiveStream, error)
	ListLive(ctx context.Context) ([]*models.LiveStream, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateStarted(ctx context.Context, id string) error
	UpdateEnded(ctx context.Context, id string) error
	UpdateHLSPath(ctx context.Context, id, hlsPath string) error
	UpdateVideoID(ctx context.Context, id, videoID string) error
	UpdateSourceInfo(ctx context.Context, id string, width, height int, codec, fps string) error
}
