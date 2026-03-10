package handler

import (
	"encoding/json"
	"net/http"

	"philos-video/internal/models"
	"philos-video/internal/repository"
)

type StreamKeyHandler struct {
	repo *repository.StreamKeyRepo
}

func NewStreamKeyHandler(repo *repository.StreamKeyRepo) *StreamKeyHandler {
	return &StreamKeyHandler{repo: repo}
}

// POST /api/v1/stream-keys
func (h *StreamKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Label == "" {
		http.Error(w, "label required", http.StatusBadRequest)
		return
	}

	sk, err := h.repo.Create(req.Label)
	if err != nil {
		http.Error(w, "failed to create stream key", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sk)
}

// GET /api/v1/stream-keys
func (h *StreamKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.repo.List()
	if err != nil {
		http.Error(w, "failed to list stream keys", http.StatusInternalServerError)
		return
	}
	if keys == nil {
		keys = []*models.StreamKey{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

// DELETE /api/v1/stream-keys/{id}
func (h *StreamKeyHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.repo.Deactivate(id); err != nil {
		http.Error(w, "failed to deactivate stream key", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
