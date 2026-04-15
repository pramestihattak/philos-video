package chat

import (
	"context"

	"philos-video/internal/models"
)

// Hubber is the interface for live chat fan-out operations.
type Hubber interface {
	Subscribe(streamID string) (chan *models.ChatMessage, error)
	Unsubscribe(streamID string, ch chan *models.ChatMessage)
	Send(ctx context.Context, streamID, userID, userName, userPic, body string) (*models.ChatMessage, error)
	GetHistory(ctx context.Context, streamID string, limit int) ([]*models.ChatMessage, error)
}
