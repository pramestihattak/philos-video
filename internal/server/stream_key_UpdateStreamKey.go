package server

import (
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
)

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
