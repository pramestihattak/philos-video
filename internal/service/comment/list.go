package comment

import (
	"context"

	"philos-video/internal/models"
)

func (s *Service) ListComments(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error) {
	if limit <= 0 || limit > maxComments {
		limit = maxComments
	}
	if offset < 0 {
		offset = 0
	}
	return s.comments.ListByVideo(ctx, videoID, limit, offset)
}
