package server

import (
	"log/slog"
	"net/http"

	"philos-video/gen/api"
)

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
		writeJSON(w, []api.ResponseChatMessage{}, http.StatusOK)
		return
	}
	writeJSON(w, toResponseChatMessages(msgs), http.StatusOK)
}
