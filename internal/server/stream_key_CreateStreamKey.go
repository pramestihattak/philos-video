package server

import (
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
)

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
	writeJSON(w, toResponseStreamKey(sk), http.StatusCreated)
}
