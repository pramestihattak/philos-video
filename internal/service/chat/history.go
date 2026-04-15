package chat

import (
	"context"

	"philos-video/internal/models"
)

// GetHistory returns persisted messages for a stream (for initial load / VOD replay).
func (h *Hub) GetHistory(ctx context.Context, streamID string, limit int) ([]*models.ChatMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return h.repo.ListByStream(ctx, streamID, limit, 0)
}
