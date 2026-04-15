package chat

import (
	"context"

	"philos-video/internal/models"
)

// Repository defines the persistence operations for live chat messages.
type Repository interface {
	Create(ctx context.Context, m *models.ChatMessage) error
	ListByStream(ctx context.Context, streamID string, limit, offset int) ([]*models.ChatMessage, error)
}
