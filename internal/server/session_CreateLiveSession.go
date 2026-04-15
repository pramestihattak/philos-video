package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"philos-video/gen/api"
)

// CreateLiveSession handles POST /api/v1/live/{stream_id}/sessions.
func (s *Server) CreateLiveSession(w http.ResponseWriter, r *http.Request, streamId string) {
	stream, err := s.liveMgr.GetStream(streamId)
	if err != nil || stream == nil {
		writeError(w, "stream not found", http.StatusNotFound)
		return
	}

	var req struct {
		DeviceType string `json:"device_type"`
		UserAgent  string `json:"user_agent"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	ua := req.UserAgent
	if ua == "" {
		ua = r.Header.Get("User-Agent")
	}
	ip := clientIP(r)

	session, token, expiresAt, err := s.sessionSvc.CreateLiveSession(r.Context(), streamId, req.DeviceType, ua, ip)
	if err != nil {
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	manifestURL := fmt.Sprintf("/live/%s/master.m3u8?token=%s", streamId, token)
	telemetryURL := fmt.Sprintf("/api/v1/sessions/%s/events", session.ID)
	_ = expiresAt

	writeJSON(w, api.ResponsePlaybackSession{
		SessionId:    session.ID,
		ManifestUrl:  manifestURL,
		Token:        token,
		TelemetryUrl: &telemetryURL,
	}, http.StatusCreated)
}
