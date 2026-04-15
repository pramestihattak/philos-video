package video

import (
	"context"

	"philos-video/internal/models"
)

// Servicer is the interface for video management operations.
type Servicer interface {
	GetVideo(ctx context.Context, id string) (*models.Video, error)
	ListVideos(ctx context.Context, limit, offset int, userID string) ([]*models.Video, error)
	GetVideoStatus(ctx context.Context, id string) (*VideoStatus, error)
	DeleteVideo(ctx context.Context, id, userID string) error
	UpdateVisibility(ctx context.Context, id, userID, visibility string) error
}
