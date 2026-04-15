package server

import (
	"log/slog"
	"net/http"

	"philos-video/gen/api"
	"philos-video/internal/middleware"
)

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
		writeJSON(w, []api.ResponseStreamKey{}, http.StatusOK)
		return
	}
	writeJSON(w, toResponseStreamKeys(keys), http.StatusOK)
}
