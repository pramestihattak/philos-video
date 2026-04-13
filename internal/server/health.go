package server

import (
	"net/http"
)

// GetHealth handles GET /health (liveness probe).
func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.healthChecker.Liveness(), http.StatusOK)
}

// GetHealthReady handles GET /health/ready (readiness probe).
func (s *Server) GetHealthReady(w http.ResponseWriter, r *http.Request) {
	checks, healthy := s.healthChecker.Readiness(r.Context())

	status := "ok"
	if !healthy {
		status = "degraded"
	}

	code := http.StatusOK
	if !healthy {
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, map[string]any{
		"status": status,
		"checks": checks,
	}, code)
}
