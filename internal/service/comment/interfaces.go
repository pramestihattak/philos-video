package comment

import (
	"context"

	"philos-video/internal/models"
)

// Servicer is the interface for comment operations.
type Servicer interface {
	AddComment(ctx context.Context, videoID, userID, userName, userPic, body string) (*models.Comment, error)
	ListComments(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error)
	DeleteComment(ctx context.Context, commentID, userID string) error
}
