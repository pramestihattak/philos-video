package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"philos-video/internal/models"
	"philos-video/internal/qoe"
	"philos-video/internal/repository"
)

type TelemetryHandler struct {
	sessions *repository.SessionRepo
	events   *repository.EventRepo
	agg      *qoe.Aggregator
}

func NewTelemetryHandler(sessions *repository.SessionRepo, events *repository.EventRepo, agg *qoe.Aggregator) *TelemetryHandler {
	return &TelemetryHandler{sessions: sessions, events: events, agg: agg}
}

// POST /api/v1/sessions/{session_id}/events
func (h *TelemetryHandler) PostEvents(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")

	session, err := h.sessions.Get(r.Context(), sessionID)
	if err != nil || session == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if session.Status != "active" {
		http.Error(w, "session not active", http.StatusGone)
		return
	}

	var req struct {
		Events []models.PlaybackEvent `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	now := time.Now()
	for i := range req.Events {
		req.Events[i].SessionID = sessionID
		req.Events[i].VideoID = session.VideoID
		if req.Events[i].Timestamp.IsZero() {
			req.Events[i].Timestamp = now
		}
		// Mark session ended if player signals end
		if req.Events[i].EventType == "playback_end" {
			go h.sessions.MarkEnded(r.Context(), sessionID)
		}
	}

	if err := h.events.BatchInsert(r.Context(), req.Events); err != nil {
		slog.Error("batch insert events", "session_id", sessionID, "err", err)
		http.Error(w, "failed to store events", http.StatusInternalServerError)
		return
	}

	h.agg.Ingest(req.Events)

	slog.Info("events received",
		slog.String("session_id", sessionID),
		slog.Int("event_count", len(req.Events)),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"received": len(req.Events)})
}
