package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
)

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
