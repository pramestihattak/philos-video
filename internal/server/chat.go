package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"philos-video/internal/api"
	"philos-video/internal/middleware"
	"philos-video/internal/models"
	"philos-video/internal/service"
)

// SendChatMessage handles POST /api/v1/live/{stream_id}/chat.
func (s *Server) SendChatMessage(w http.ResponseWriter, r *http.Request, streamId string) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	msg, err := s.chatHub.Send(r.Context(), streamId, user.ID, user.Name, user.Picture, req.Body)
	if err != nil {
		var ve *service.ValidationError
		if errors.As(err, &ve) {
			writeError(w, ve.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("send chat message", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, msg, http.StatusCreated)
}

// ChatStream handles GET /api/v1/live/{stream_id}/chat/stream (SSE).
func (s *Server) ChatStream(w http.ResponseWriter, r *http.Request, streamId string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	history, err := s.chatHub.GetHistory(r.Context(), streamId, 100)
	if err != nil {
		slog.Error("chat history", "stream_id", streamId, "err", err)
	}
	if history == nil {
		history = []*models.ChatMessage{}
	}
	if data, err := json.Marshal(map[string]any{"history": history}); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	ch, err := s.chatHub.Subscribe(streamId)
	if err != nil {
		writeError(w, "too many connections", http.StatusServiceUnavailable)
		return
	}
	defer s.chatHub.Unsubscribe(streamId, ch)

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

// ListChatMessages handles GET /api/v1/live/{stream_id}/chat.
func (s *Server) ListChatMessages(w http.ResponseWriter, r *http.Request, streamId string, params api.ListChatMessagesParams) {
	limit := 50
	offset := 0
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Offset != nil {
		offset = *params.Offset
	}

	msgs, err := s.chatHub.GetHistory(r.Context(), streamId, limit)
	_ = offset // history uses limit; offset support can be added to ChatHub later
	if err != nil {
		slog.Error("list chat messages", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []*models.ChatMessage{}
	}
	writeJSON(w, msgs, http.StatusOK)
}
