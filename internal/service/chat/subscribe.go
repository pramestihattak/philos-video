package chat

import (
	"fmt"

	"philos-video/internal/models"
)

// Subscribe returns a buffered channel that receives messages for streamID.
// Returns an error if the room has reached maxSubscribers.
// The caller must call Unsubscribe when done.
func (h *Hub) Subscribe(streamID string) (chan *models.ChatMessage, error) {
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
func (h *Hub) Unsubscribe(streamID string, ch chan *models.ChatMessage) {
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
