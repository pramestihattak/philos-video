package server

import "net/http"

// GetHealth handles GET /health (liveness probe).
func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.healthChecker.Liveness(), http.StatusOK)
}
