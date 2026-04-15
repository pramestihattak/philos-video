package server

import (
	"log/slog"
	"net/http"

	"philos-video/gen/api"
	"philos-video/internal/middleware"
)

// ListVideos handles GET /api/v1/videos.
func (s *Server) ListVideos(w http.ResponseWriter, r *http.Request, params api.ListVideosParams) {
	user := middleware.CurrentUser(r.Context())

	userID := ""
	if user != nil {
		userID = user.ID
	}

	limit := 20
	page := 1
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Page != nil {
		page = *params.Page
	}
	offset := (page - 1) * limit

	videos, err := s.videoSvc.ListVideos(r.Context(), limit, offset, userID)
	if err != nil {
		slog.Error("list videos", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	if videos == nil {
		writeJSON(w, []api.ResponseVideo{}, http.StatusOK)
		return
	}
	writeJSON(w, toResponseVideos(videos), http.StatusOK)
}
