package handler

import (
	"encoding/json"
	"net/http"

	"philos-video/internal/service"
)

type VideoHandler struct {
	svc *service.VideoService
}

func NewVideoHandler(svc *service.VideoService) *VideoHandler {
	return &VideoHandler{svc: svc}
}

// GET /api/v1/videos
func (h *VideoHandler) ListVideos(w http.ResponseWriter, r *http.Request) {
	videos, err := h.svc.ListVideos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if vs == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vs)
}
