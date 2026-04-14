package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"

	"philos-video/internal/models"
	"philos-video/internal/storage"
)

const (
	maxChatLen     = 500
	maxSubscribers = 500
)

// ChatHub fans out live chat messages to SSE subscribers, keyed by stream ID.
// Incoming messages are persisted to the DB before being broadcast.
type ChatHub struct {
	mu    sync.Mutex
	rooms map[string]map[chan *models.ChatMessage]struct{}
	repo  storage.ChatMessageStorer
}

func NewChatHub(repo storage.ChatMessageStorer) *ChatHub {
	return &ChatHub{
		rooms: make(map[string]map[chan *models.ChatMessage]struct{}),
		repo:  repo,
	}
}

// Subscribe returns a buffered channel that receives messages for streamID.
// Returns an error if the room has reached maxSubscribers.
// The caller must call Unsubscribe when done.
func (h *ChatHub) Subscribe(streamID string) (chan *models.ChatMessage, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[streamID] == nil {
		h.rooms[streamID] = make(map[chan *models.ChatMessage]struct{})
	}
	if len(h.rooms[streamID]) >= maxSubscribers {
		return nil, fmt.Errorf("too many connections")
	}
	ch := make(chan *models.ChatMessage, 8)
	h.rooms[streamID][ch] = struct{}{}
	return ch, nil
}

// Unsubscribe removes ch from the room and closes it.
// Safe to call even if ch was already removed.
func (h *ChatHub) Unsubscribe(streamID string, ch chan *models.ChatMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room, ok := h.rooms[streamID]; ok {
		if _, found := room[ch]; found {
			delete(room, ch)
			close(ch)
		}
		if len(room) == 0 {
			delete(h.rooms, streamID)
		}
	}
}

// Send validates and persists a message, then broadcasts it to all subscribers.
func (h *ChatHub) Send(ctx context.Context, streamID, userID, userName, userPic, body string) (*models.ChatMessage, error) {
	if len(body) == 0 {
		return nil, validationErrorf("message cannot be empty")
	}
	if len([]rune(body)) > maxChatLen {
		return nil, validationErrorf("message exceeds %d characters", maxChatLen)
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
		return nil, err
	}

	h.broadcast(streamID, msg)
	return msg, nil
}

// GetHistory returns persisted messages for a stream (for initial load / VOD replay).
func (h *ChatHub) GetHistory(ctx context.Context, streamID string, limit int) ([]*models.ChatMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return h.repo.ListByStream(ctx, streamID, limit, 0)
}

// broadcast copies the subscriber list under the lock, releases it, then sends
// to avoid blocking Subscribe/Unsubscribe while iterating channels.
func (h *ChatHub) broadcast(streamID string, msg *models.ChatMessage) {
	h.mu.Lock()
	room := h.rooms[streamID]
	subs := make([]chan *models.ChatMessage, 0, len(room))
	for ch := range room {
		subs = append(subs, ch)
	}
	h.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
			slog.Warn("chat: dropping message for slow subscriber", "stream_id", streamID)
		}
	}
}
