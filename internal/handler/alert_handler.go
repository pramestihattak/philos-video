package handler

import (
	"encoding/json"
	"net/http"

	"philos-video/internal/alerting"
)

// AlertHandler serves /api/v1/alerts/*.
type AlertHandler struct {
	engine *alerting.Engine
}

// NewAlertHandler creates an AlertHandler.
func NewAlertHandler(engine *alerting.Engine) *AlertHandler {
	return &AlertHandler{engine: engine}
}

// Active handles GET /api/v1/alerts/active.
func (h *AlertHandler) Active(w http.ResponseWriter, r *http.Request) {
	alerts := h.engine.ActiveAlerts()
	if alerts == nil {
		alerts = []*alerting.Alert{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"alerts": alerts})
}

// History handles GET /api/v1/alerts/history.
func (h *AlertHandler) History(w http.ResponseWriter, r *http.Request) {
	alerts := h.engine.History()
	if alerts == nil {
		alerts = []*alerting.Alert{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"alerts": alerts})
}
