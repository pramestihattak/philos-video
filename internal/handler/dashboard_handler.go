package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"philos-video/internal/qoe"
)

type DashboardHandler struct {
	agg *qoe.Aggregator
}

func NewDashboardHandler(agg *qoe.Aggregator) *DashboardHandler {
	return &DashboardHandler{agg: agg}
}

// GET /api/v1/dashboard/stats
func (h *DashboardHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.agg.GetMetrics())
}

// GET /api/v1/dashboard/stats/stream  — SSE stream of live metrics
func (h *DashboardHandler) StatsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send current snapshot immediately
	if data, err := json.Marshal(h.agg.GetMetrics()); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	ch := h.agg.Subscribe()
	defer h.agg.Unsubscribe(ch)

	for {
		select {
		case metrics, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(metrics)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
