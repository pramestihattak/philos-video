package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"philos-video/internal/service"
)

// clientIP extracts the real client IP. X-Forwarded-For is only trusted when the
// direct connection arrives from a private/loopback address (i.e. a local proxy).
func clientIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}
	if isPrivateIP(remoteHost) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the leftmost (client) address from the chain.
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
	privateRanges := []string{"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "::1/128", "fc00::/7"}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

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
	ip := clientIP(r)

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
