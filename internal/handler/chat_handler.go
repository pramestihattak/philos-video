package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"philos-video/internal/middleware"
	"philos-video/internal/models"
	"philos-video/internal/service"
)

type ChatHandler struct {
	hub *service.ChatHub
}

func NewChatHandler(hub *service.ChatHub) *ChatHandler {
	return &ChatHandler{hub: hub}
}

// POST /api/v1/live/{stream_id}/chat
func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	streamID := r.PathValue("stream_id")

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	msg, err := h.hub.Send(r.Context(), streamID, user.ID, user.Name, user.Picture, req.Body)
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			http.Error(w, ve.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("send chat message", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(msg)
}

// GET /api/v1/live/{stream_id}/chat/stream — SSE endpoint
func (h *ChatHandler) ChatStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	streamID := r.PathValue("stream_id")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send recent message history as the initial batch.
	history, err := h.hub.GetHistory(r.Context(), streamID, 100)
	if err != nil {
		slog.Error("chat history", "stream_id", streamID, "err", err)
	}
	if history == nil {
		history = []*models.ChatMessage{}
	}
	if data, err := json.Marshal(map[string]any{"history": history}); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	ch, err := h.hub.Subscribe(streamID)
	if err != nil {
		http.Error(w, "too many connections", http.StatusServiceUnavailable)
		return
	}
	defer h.hub.Unsubscribe(streamID, ch)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// GET /api/v1/live/{stream_id}/chat — REST history (for VOD replay)
func (h *ChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("stream_id")

	msgs, err := h.hub.GetHistory(r.Context(), streamID, 200)
	if err != nil {
		slog.Error("list chat messages", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []*models.ChatMessage{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}
