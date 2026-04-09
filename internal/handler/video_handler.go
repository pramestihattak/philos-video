package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"philos-video/internal/middleware"
	"philos-video/internal/models"
	"philos-video/internal/service"
)

type VideoHandler struct {
	svc *service.VideoService
}

func NewVideoHandler(svc *service.VideoService) *VideoHandler {
	return &VideoHandler{svc: svc}
}

// GET /api/v1/videos?page=1&limit=20
// Returns the signed-in user's videos, or public/unlisted videos for guests.
func (h *VideoHandler) ListVideos(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())

	userID := ""
	if user != nil {
		userID = user.ID
	}

	limit := service.DefaultVideoPageLimit
	page := 1

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	offset := (page - 1) * limit

	videos, err := h.svc.ListVideos(limit, offset, userID)
	if err != nil {
		slog.Error("list videos", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(videos) == 0 {
		w.Write([]byte("[]"))
		return
	}
	json.NewEncoder(w).Encode(videos)
}

// GET /api/v1/videos/{id}
// Public for unlisted/public; owner-only for private.
func (h *VideoHandler) GetVideo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	video, err := h.svc.GetVideo(id)
	if err != nil {
		slog.Error("get video", "id", id, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if video == nil {
		http.NotFound(w, r)
		return
	}
	if video.Visibility == models.VisibilityPrivate {
		user := middleware.CurrentUser(r.Context())
		if user == nil || user.ID != video.UserID {
			http.NotFound(w, r)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(video)
}

// GET /api/v1/videos/{id}/status
func (h *VideoHandler) GetVideoStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	vs, err := h.svc.GetVideoStatus(id)
	if err != nil {
		slog.Error("get video status", "id", id, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if vs == nil {
		http.NotFound(w, r)
		return
	}

	// Apply same visibility rule as GetVideo.
	if vs.Video.Visibility == models.VisibilityPrivate {
		user := middleware.CurrentUser(r.Context())
		if user == nil || user.ID != vs.Video.UserID {
			http.NotFound(w, r)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vs)
}

// DELETE /api/v1/videos/{id}
func (h *VideoHandler) DeleteVideo(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	if err := h.svc.DeleteVideo(r.Context(), id, user.ID); err != nil {
		slog.Error("delete video", "id", id, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PATCH /api/v1/videos/{id}
// Supports updating visibility.
func (h *VideoHandler) UpdateVideo(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")

	var req struct {
		Visibility *string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Visibility != nil {
		if err := h.svc.UpdateVisibility(r.Context(), id, user.ID, *req.Visibility); err != nil {
			slog.Error("update visibility", "id", id, "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
