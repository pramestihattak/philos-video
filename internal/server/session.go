package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
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

	writeJSON(w, map[string]any{
		"session_id":       session.ID,
		"manifest_url":     manifestURL,
		"token":            token,
		"token_expires_at": expiresAt,
		"telemetry_url":    telemetryURL,
	}, http.StatusCreated)
}

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

	writeJSON(w, map[string]any{
		"session_id":       session.ID,
		"manifest_url":     manifestURL,
		"token":            token,
		"token_expires_at": expiresAt,
		"telemetry_url":    telemetryURL,
	}, http.StatusCreated)
}

// clientIP extracts the real client IP.
func clientIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}
	if isPrivateIP(remoteHost) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
		}
	}
	return remoteHost
}

func isPrivateIP(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range []string{"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "::1/128", "fc00::/7"} {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}
