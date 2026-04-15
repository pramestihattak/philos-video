package chat

import (
	"log/slog"
	"sync"

	"philos-video/internal/models"
	chatrepo "philos-video/internal/storage/chat"
)

const (
	maxChatLen     = 500
	maxSubscribers = 500
)

// Hub fans out live chat messages to SSE subscribers, keyed by stream ID.
// Incoming messages are persisted to the DB before being broadcast.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]map[chan *models.ChatMessage]struct{}
	repo  chatrepo.Repository
}

// New creates a chat Hub.
func New(repo chatrepo.Repository) *Hub {
	return &Hub{
		rooms: make(map[string]map[chan *models.ChatMessage]struct{}),
		repo:  repo,
	}
}

// broadcast copies the subscriber list under the lock, releases it, then sends
// to avoid blocking Subscribe/Unsubscribe while iterating channels.
func (h *Hub) broadcast(streamID string, msg *models.ChatMessage) {
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
