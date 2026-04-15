package server

import (
	"log/slog"
	"net/http"

	"philos-video/internal/middleware"
)

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
