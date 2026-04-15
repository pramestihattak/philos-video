package server

import (
	"net/http"

	"philos-video/internal/middleware"
)

// GetMe handles GET /api/v1/me.
func (s *Server) GetMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.CurrentUser(r.Context())
	if user == nil {
		writeError(w, "not signed in", http.StatusUnauthorized)
		return
	}
	writeJSON(w, toResponseUser(user), http.StatusOK)
}
