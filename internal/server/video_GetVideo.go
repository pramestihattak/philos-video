package server

import (
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
	"philos-video/internal/models"
)

// GetVideo handles GET /api/v1/videos/{id}.
func (s *Server) GetVideo(w http.ResponseWriter, r *http.Request, id string) {
	video, err := s.videoSvc.GetVideo(r.Context(), id)
	if err != nil {
		slog.Error("get video", "id", id, "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if video == nil {
		writeError(w, "not found", http.StatusNotFound)
		return
	}
	if video.Visibility == models.VisibilityPrivate {
		user := middleware.CurrentUser(r.Context())
		if user == nil || user.ID != video.UserID {
			writeError(w, "not found", http.StatusNotFound)
			return
		}
	}
	writeJSON(w, toResponseVideo(video), http.StatusOK)
}
