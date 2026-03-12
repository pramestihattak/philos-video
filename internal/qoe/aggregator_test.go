package qoe_test

import (
	"testing"
	"time"

	"philos-video/internal/models"
	"philos-video/internal/qoe"
)

func TestAggregator_Ingest_HeartbeatTracksActiveSessions(t *testing.T) {
	// Aggregator needs a nil videoRepo — OK for unit tests that don't trigger title fetching.
	agg := qoe.New(nil)

	events := []models.PlaybackEvent{
		{SessionID: "sess1", VideoID: "vid1", EventType: "heartbeat", CurrentQuality: "720p"},
		{SessionID: "sess2", VideoID: "vid1", EventType: "heartbeat", CurrentQuality: "480p"},
	}
	agg.Ingest(events)

	// Give the background loop a moment to recalculate.
	time.Sleep(1100 * time.Millisecond)

	m := agg.GetMetrics()
	if m.ActiveSessions != 2 {
		t.Errorf("ActiveSessions = %d, want 2", m.ActiveSessions)
	}
}

func TestAggregator_Subscribe_ReceivesUpdate(t *testing.T) {
	agg := qoe.New(nil)
	ch := agg.Subscribe()
	defer agg.Unsubscribe(ch)

	agg.Ingest([]models.PlaybackEvent{
		{SessionID: "sess3", VideoID: "vid2", EventType: "heartbeat", CurrentQuality: "360p"},
	})

	select {
	case m := <-ch:
		if m == nil {
			t.Fatal("received nil metrics")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: no metrics update received")
	}
}

func TestAggregator_GetMetrics_EmptyIsNotNil(t *testing.T) {
	agg := qoe.New(nil)
	m := agg.GetMetrics()
	if m == nil {
		t.Fatal("GetMetrics returned nil for fresh aggregator")
	}
	if m.QualityDistribution == nil {
		t.Fatal("QualityDistribution should be initialised, not nil")
	}
}
