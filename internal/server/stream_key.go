package server

import (
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
	"philos-video/internal/models"
)

func (s *Server) canGoLive(user *models.User) bool {
	return middleware.CanGoLive(s.goLiveWhitelist, user.Email)
}

// ListStreamKeys handles GET /api/v1/stream-keys.
func (s *Server) ListStreamKeys(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.canGoLive(user) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}

	keys, err := s.streamKeyRepo.List(r.Context(), user.ID)
	if err != nil {
		slog.Error("list stream keys", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if keys == nil {
		keys = []*models.StreamKey{}
	}
	writeJSON(w, keys, http.StatusOK)
}

// CreateStreamKey handles POST /api/v1/stream-keys.
func (s *Server) CreateStreamKey(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.canGoLive(user) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Label     string `json:"label"`
		RecordVOD *bool  `json:"record_vod"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Label == "" {
		writeError(w, "label is required", http.StatusBadRequest)
		return
	}

	recordVOD := true
	if req.RecordVOD != nil {
		recordVOD = *req.RecordVOD
	}

	sk, err := s.streamKeyRepo.Create(r.Context(), req.Label, recordVOD, user.ID)
	if err != nil {
		slog.Error("create stream key", "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, sk, http.StatusCreated)
}

// DeactivateStreamKey handles DELETE /api/v1/stream-keys/{id}.
func (s *Server) DeactivateStreamKey(w http.ResponseWriter, r *http.Request, id string) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.canGoLive(user) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.streamKeyRepo.Deactivate(r.Context(), id, user.ID); err != nil {
		slog.Error("deactivate stream key", "id", id, "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateStreamKey handles PATCH /api/v1/stream-keys/{id}.
func (s *Server) UpdateStreamKey(w http.ResponseWriter, r *http.Request, id string) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.canGoLive(user) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		RecordVOD *bool `json:"record_vod"`
	}
	if err := decodeJSON(r, &req); err != nil || req.RecordVOD == nil {
		writeError(w, "record_vod is required", http.StatusBadRequest)
		return
	}

	if err := s.streamKeyRepo.UpdateRecordVOD(r.Context(), id, *req.RecordVOD, user.ID); err != nil {
		slog.Error("update stream key", "id", id, "err", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
