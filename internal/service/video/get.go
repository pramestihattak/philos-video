package video

import (
	"context"

	"philos-video/internal/models"
)

func (s *Service) GetVideo(ctx context.Context, id string) (*models.Video, error) {
	return s.videos.GetByID(ctx, id)
}
