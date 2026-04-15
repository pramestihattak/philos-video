package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"philos-video/gen/api"
)

// CreateVideoSession handles POST /api/v1/videos/{id}/sessions.
func (s *Server) CreateVideoSession(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		DeviceType string `json:"device_type"`
		UserAgent  string `json:"user_agent"`
	}
	json.NewDecoder(r.Body).Decode(&req) // best-effort; body may be empty

	ua := req.UserAgent
	if ua == "" {
		ua = r.Header.Get("User-Agent")
	}
	ip := clientIP(r)

	session, token, expiresAt, err := s.sessionSvc.CreateSession(r.Context(), id, req.DeviceType, ua, ip)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	manifestURL := fmt.Sprintf("/videos/%s/master.m3u8?token=%s", id, token)
	telemetryURL := fmt.Sprintf("/api/v1/sessions/%s/events", session.ID)
	_ = expiresAt

	writeJSON(w, api.ResponsePlaybackSession{
		SessionId:    session.ID,
		ManifestUrl:  manifestURL,
		Token:        token,
		TelemetryUrl: &telemetryURL,
	}, http.StatusCreated)
}
