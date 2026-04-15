package video

import (
	"context"

	"philos-video/internal/models"
)

// Repository defines the persistence operations for videos.
type Repository interface {
	Create(ctx context.Context, v *models.Video) error
	GetByID(ctx context.Context, id string) (*models.Video, error)
	GetByIDForUser(ctx context.Context, id, userID string) (*models.Video, error)
	ListPublic(ctx context.Context, limit, offset int) ([]*models.Video, error)
	List(ctx context.Context, limit, offset int, userID string) ([]*models.Video, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateAfterProbe(ctx context.Context, id string, width, height int, duration, codec string) error
	UpdateHLSPath(ctx context.Context, id, hlsPath string) error
	UpdateSizeBytes(ctx context.Context, id string, size int64) error
	UpdateThumbnailPath(ctx context.Context, id, thumbnailPath string) error
	UpdateVisibility(ctx context.Context, id, userID, visibility string) error
	Delete(ctx context.Context, id, userID string) error
}
