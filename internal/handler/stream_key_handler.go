package handler

import (
	"encoding/json"
	"net/http"

	"philos-video/internal/middleware"
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
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Label     string `json:"label"`
		RecordVOD *bool  `json:"record_vod"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Label == "" {
		http.Error(w, "label required", http.StatusBadRequest)
		return
	}

	recordVOD := true
	if req.RecordVOD != nil {
		recordVOD = *req.RecordVOD
	}

	sk, err := h.repo.Create(req.Label, recordVOD, user.ID)
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
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	keys, err := h.repo.List(user.ID)
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
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	if err := h.repo.Deactivate(id, user.ID); err != nil {
		http.Error(w, "failed to deactivate stream key", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PATCH /api/v1/stream-keys/{id}
func (h *StreamKeyHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	var req struct {
		RecordVOD *bool `json:"record_vod"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RecordVOD == nil {
		http.Error(w, "record_vod required", http.StatusBadRequest)
		return
	}
	if err := h.repo.UpdateRecordVOD(id, *req.RecordVOD, user.ID); err != nil {
		http.Error(w, "failed to update stream key", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
