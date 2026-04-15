package server

import (
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
	"philos-video/internal/models"
)

// GetVideoStatus handles GET /api/v1/videos/{id}/status.
func (s *Server) GetVideoStatus(w http.ResponseWriter, r *http.Request, id string) {
	vs, err := s.videoSvc.GetVideoStatus(r.Context(), id)
	if err != nil {
		slog.Error("get video status", "id", id, "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if vs == nil {
		writeError(w, "not found", http.StatusNotFound)
		return
	}
	if vs.Video.Visibility == models.VisibilityPrivate {
		user := middleware.CurrentUser(r.Context())
		if user == nil || user.ID != vs.Video.UserID {
			writeError(w, "not found", http.StatusNotFound)
			return
		}
	}
	writeJSON(w, toResponseVideoStatus(vs), http.StatusOK)
}
