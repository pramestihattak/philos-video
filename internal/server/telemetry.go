package server

import (
	"log/slog"
	"net/http"
	"time"

	"philos-video/internal/metrics"
	"philos-video/internal/models"
)

// PostTelemetryEvents handles POST /api/v1/sessions/{session_id}/events.
func (s *Server) PostTelemetryEvents(w http.ResponseWriter, r *http.Request, sessionId string) {
	session, err := s.sessionRepo.Get(r.Context(), sessionId)
	if err != nil || session == nil {
		writeError(w, "session not found", http.StatusNotFound)
		return
	}
	if session.Status != "active" {
		writeError(w, "session not active", http.StatusGone)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB cap
	var req struct {
		Events []models.PlaybackEvent `json:"events"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(req.Events) > 1000 {
		writeError(w, "too many events", http.StatusBadRequest)
		return
	}

	now := time.Now()
	for i := range req.Events {
		req.Events[i].SessionID = sessionId
		req.Events[i].VideoID = session.VideoID
		if req.Events[i].Timestamp.IsZero() {
			req.Events[i].Timestamp = now
		}
		if req.Events[i].EventType == "playback_end" {
			go s.sessionRepo.MarkEnded(r.Context(), sessionId)
		}
	}

	if err := s.eventRepo.BatchInsert(r.Context(), req.Events); err != nil {
		slog.Error("batch insert events", "session_id", sessionId, "err", err)
		writeError(w, "failed to store events", http.StatusInternalServerError)
		return
	}

	// Bridge to Prometheus metrics
	for _, e := range req.Events {
		metrics.TelemetryEventsReceived.WithLabelValues(e.EventType).Inc()
		switch e.EventType {
		case "playback_start":
			if e.DownloadTimeMs != nil && *e.DownloadTimeMs > 0 {
				metrics.PlaybackTTFFSeconds.Observe(float64(*e.DownloadTimeMs) / 1000)
			}
		case "rebuffer_start":
			metrics.PlaybackRebufferTotal.Inc()
		case "rebuffer_end":
			if e.RebufferDurationMs != nil {
				metrics.PlaybackRebufferDuration.Observe(float64(*e.RebufferDurationMs) / 1000)
			}
		case "quality_change":
			direction := "up"
			if e.QualityTo < e.QualityFrom {
				direction = "down"
			}
			metrics.PlaybackQualitySwitchesTotal.WithLabelValues(direction).Inc()
		case "playback_error":
			code := "unknown"
			if e.ErrorCode != "" {
				code = e.ErrorCode
			}
			metrics.PlaybackErrorsTotal.WithLabelValues(code).Inc()
		}
	}

	slog.Info("events received",
		slog.String("session_id", sessionId),
		slog.Int("event_count", len(req.Events)),
	)

	writeJSON(w, map[string]int{"received": len(req.Events)}, http.StatusOK)
}
