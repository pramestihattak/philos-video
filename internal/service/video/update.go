package video

import (
	"context"
	"fmt"

	"philos-video/internal/models"
)

// UpdateVisibility changes the video's visibility, scoped to its owner.
func (s *Service) UpdateVisibility(ctx context.Context, id, userID, visibility string) error {
	switch visibility {
	case models.VisibilityPrivate, models.VisibilityUnlisted, models.VisibilityPublic:
		// valid
	default:
		return fmt.Errorf("invalid visibility %q", visibility)
	}
	return s.videos.UpdateVisibility(ctx, id, userID, visibility)
}
