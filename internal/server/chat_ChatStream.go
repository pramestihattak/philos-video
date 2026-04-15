package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

)

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
	histResp := toResponseChatMessages(history)
	if data, err := json.Marshal(map[string]any{"history": histResp}); err == nil {
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
			data, err := json.Marshal(toResponseChatMessage(msg))
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
