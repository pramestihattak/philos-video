package chat

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"philos-video/internal/models"
	"philos-video/internal/service"
)

// Send validates and persists a message, then broadcasts it to all subscribers.
func (h *Hub) Send(ctx context.Context, streamID, userID, userName, userPic, body string) (*models.ChatMessage, error) {
	if len(body) == 0 {
		return nil, service.NewValidationErrorf("message cannot be empty")
	}
	if len([]rune(body)) > maxChatLen {
		return nil, service.NewValidationErrorf("message exceeds %d characters", maxChatLen)
	}

	msg := &models.ChatMessage{
		ID:       uuid.New().String(),
		StreamID: streamID,
		UserID:   userID,
		UserName: userName,
		UserPic:  userPic,
		Body:     body,
	}

	if err := h.repo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("persisting chat message: %w", err)
	}

	h.broadcast(streamID, msg)
	return msg, nil
}
