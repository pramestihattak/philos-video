package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"philos-video/internal/service"
)

type SessionHandler struct {
	svc *service.SessionService
}

func NewSessionHandler(svc *service.SessionService) *SessionHandler {
	return &SessionHandler{svc: svc}
}

// POST /api/v1/videos/{id}/sessions
func (h *SessionHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	videoID := r.PathValue("id")

	var req struct {
		DeviceType string `json:"device_type"`
		UserAgent  string `json:"user_agent"`
	}
	// best-effort decode; body may be empty
	json.NewDecoder(r.Body).Decode(&req)

	ua := req.UserAgent
	if ua == "" {
		ua = r.Header.Get("User-Agent")
	}
	ip := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = xff
	}

	session, token, expiresAt, err := h.svc.CreateSession(r.Context(), videoID, req.DeviceType, ua, ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	manifestURL := fmt.Sprintf("/videos/%s/master.m3u8?token=%s", videoID, token)
	telemetryURL := fmt.Sprintf("/api/v1/sessions/%s/events", session.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"session_id":       session.ID,
		"manifest_url":     manifestURL,
		"token":            token,
		"token_expires_at": expiresAt,
		"telemetry_url":    telemetryURL,
	})
}
