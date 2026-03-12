package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"philos-video/internal/service"
)

type VideoHandler struct {
	svc *service.VideoService
}

func NewVideoHandler(svc *service.VideoService) *VideoHandler {
	return &VideoHandler{svc: svc}
}

// GET /api/v1/videos?page=1&limit=20
func (h *VideoHandler) ListVideos(w http.ResponseWriter, r *http.Request) {
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

	videos, err := h.svc.ListVideos(limit, offset)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vs)
}

// DELETE /api/v1/videos/{id}
func (h *VideoHandler) DeleteVideo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.DeleteVideo(r.Context(), id); err != nil {
		slog.Error("delete video", "id", id, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
