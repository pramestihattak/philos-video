package server

import (
	"errors"
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
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

	writeJSON(w, toResponseChatMessage(msg), http.StatusCreated)
}
