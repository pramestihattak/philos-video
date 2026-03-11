package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"philos-video/internal/live"
	"philos-video/internal/models"
	"philos-video/internal/repository"
	"philos-video/internal/service"
)

type LiveHandler struct {
	manager     *live.Manager
	sessionSvc  *service.SessionService
	sessionRepo *repository.SessionRepo
}

func NewLiveHandler(manager *live.Manager, sessionSvc *service.SessionService, sessionRepo *repository.SessionRepo) *LiveHandler {
	return &LiveHandler{manager: manager, sessionSvc: sessionSvc, sessionRepo: sessionRepo}
}

// GET /api/v1/live
func (h *LiveHandler) ListLive(w http.ResponseWriter, r *http.Request) {
	streams, err := h.manager.ListLive()
	if err != nil {
		http.Error(w, "failed to list streams", http.StatusInternalServerError)
		return
	}
	if streams == nil {
		streams = []*models.LiveStream{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(streams)
}

// GET /api/v1/live/{stream_id}
func (h *LiveHandler) GetStream(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("stream_id")
	stream, err := h.manager.GetStream(streamID)
	if err != nil || stream == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stream)
}

// POST /api/v1/live/{stream_id}/sessions
func (h *LiveHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("stream_id")

	stream, err := h.manager.GetStream(streamID)
	if err != nil || stream == nil {
		http.Error(w, "stream not found", http.StatusNotFound)
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
	ip := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = xff
	}

	session, token, expiresAt, err := h.sessionSvc.CreateLiveSession(r.Context(), streamID, req.DeviceType, ua, ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	manifestURL := fmt.Sprintf("/live/%s/master.m3u8?token=%s", streamID, token)
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

// GET /api/v1/live/{stream_id}/viewers
func (h *LiveHandler) Viewers(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("stream_id")
	count, err := h.sessionRepo.CountActiveByStreamID(r.Context(), streamID)
	if err != nil {
		http.Error(w, "failed to count viewers", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"viewers": count})
}

// POST /api/v1/live/{stream_id}/end
func (h *LiveHandler) EndStream(w http.ResponseWriter, r *http.Request) {
	streamID := r.PathValue("stream_id")
	h.manager.EndStream(streamID)
	w.WriteHeader(http.StatusNoContent)
}
