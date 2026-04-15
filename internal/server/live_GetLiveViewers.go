package server

import (
	"log/slog"
	"net/http"
)

// GetLiveViewers handles GET /api/v1/live/{stream_id}/viewers.
func (s *Server) GetLiveViewers(w http.ResponseWriter, r *http.Request, streamId string) {
	count, err := s.sessionRepo.CountActiveByStreamID(r.Context(), streamId)
	if err != nil {
		slog.Error("count viewers", "stream_id", streamId, "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]int{"count": count}, http.StatusOK)
}
