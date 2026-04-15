package server

import (
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
)

// DeleteVideo handles DELETE /api/v1/videos/{id}.
func (s *Server) DeleteVideo(w http.ResponseWriter, r *http.Request, id string) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := s.videoSvc.DeleteVideo(r.Context(), id, user.ID); err != nil {
		slog.Error("delete video", "id", id, "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
