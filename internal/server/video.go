package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"philos-video/internal/api"
	"philos-video/internal/middleware"
	"philos-video/internal/models"
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

	videos, err := s.videoSvc.ListVideos(limit, offset, userID)
	if err != nil {
		slog.Error("list videos", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	if videos == nil {
		videos = []*models.Video{}
	}
	writeJSON(w, videos, http.StatusOK)
}

// GetVideo handles GET /api/v1/videos/{id}.
func (s *Server) GetVideo(w http.ResponseWriter, r *http.Request, id string) {
	video, err := s.videoSvc.GetVideo(id)
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
	writeJSON(w, video, http.StatusOK)
}

// GetVideoStatus handles GET /api/v1/videos/{id}/status.
func (s *Server) GetVideoStatus(w http.ResponseWriter, r *http.Request, id string) {
	vs, err := s.videoSvc.GetVideoStatus(id)
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
	writeJSON(w, vs, http.StatusOK)
}

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

// UpdateVideo handles PATCH /api/v1/videos/{id}.
func (s *Server) UpdateVideo(w http.ResponseWriter, r *http.Request, id string) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Visibility *string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Visibility != nil {
		if err := s.videoSvc.UpdateVisibility(r.Context(), id, user.ID, *req.Visibility); err != nil {
			slog.Error("update visibility", "id", id, "err", err)
			writeError(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
