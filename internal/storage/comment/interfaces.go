package comment

import (
	"context"

	"philos-video/internal/models"
)

// Repository defines the persistence operations for comments.
type Repository interface {
	Create(ctx context.Context, c *models.Comment) error
	ListByVideo(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error)
	Delete(ctx context.Context, id, userID string) error
	DeleteByVideo(ctx context.Context, videoID string) error
}
