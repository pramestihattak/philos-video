package server

import (
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
	"philos-video/internal/models"
)

// ListLiveStreams handles GET /api/v1/live.
func (s *Server) ListLiveStreams(w http.ResponseWriter, r *http.Request) {
	streams, err := s.liveMgr.ListLive()
	if err != nil {
		writeError(w, "failed to list streams", http.StatusInternalServerError)
		return
	}
	if streams == nil {
		streams = []*models.LiveStream{}
	}
	writeJSON(w, streams, http.StatusOK)
}

// GetLiveStream handles GET /api/v1/live/{stream_id}.
func (s *Server) GetLiveStream(w http.ResponseWriter, r *http.Request, streamId string) {
	stream, err := s.liveMgr.GetStream(streamId)
	if err != nil || stream == nil {
		writeError(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, stream, http.StatusOK)
}

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

	s.liveMgr.EndStream(streamId)
	w.WriteHeader(http.StatusNoContent)
}
