package video

import (
	"context"

	"philos-video/internal/models"
)

// ListVideos returns videos for a user (owner-scoped) or public/unlisted videos if userID is empty.
func (s *Service) ListVideos(ctx context.Context, limit, offset int, userID string) ([]*models.Video, error) {
	if limit <= 0 {
		limit = DefaultPageLimit
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}
	if userID == "" {
		return s.videos.ListPublic(ctx, limit, offset)
	}
	return s.videos.List(ctx, limit, offset, userID)
}
