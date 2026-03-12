package handler

import (
	"encoding/json"
	"net/http"

	"philos-video/internal/health"
)

// HealthHandler serves /health and /health/ready.
type HealthHandler struct {
	checker *health.HealthChecker
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(checker *health.HealthChecker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

// Liveness handles GET /health — always 200 with uptime.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.checker.Liveness())
}

// Readiness handles GET /health/ready — 200 if all checks pass, 503 otherwise.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	checks, healthy := h.checker.Readiness(r.Context())

	status := "ok"
	if !healthy {
		status = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	if !healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(map[string]any{
		"status": status,
		"checks": checks,
	})
}
