package event

import (
	"context"

	"philos-video/internal/models"
)

// Repository defines the persistence operations for playback events.
type Repository interface {
	BatchInsert(ctx context.Context, events []models.PlaybackEvent) error
}
