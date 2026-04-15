package server

import (
	"net/http"

	"philos-video/internal/middleware"
	"philos-video/internal/models"
)

// EndLiveStream handles POST /api/v1/live/{stream_id}/end.
func (s *Server) EndLiveStream(w http.ResponseWriter, r *http.Request, streamId string) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	stream, err := s.liveMgr.GetStream(streamId)
	if err != nil || stream == nil || stream.UserID != user.ID {
		writeError(w, "not found", http.StatusNotFound)
		return
	}
	if stream.Status != models.StreamStatusLive && stream.Status != models.StreamStatusWaiting {
		writeError(w, "stream is not active", http.StatusConflict)
		return
	}

	s.liveMgr.EndStream(streamId)
	w.WriteHeader(http.StatusNoContent)
}
