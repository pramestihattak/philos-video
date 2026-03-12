package alerting

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"philos-video/internal/qoe"
)

// AlertSeverity is the severity level of an alert.
type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityWarning  AlertSeverity = "warning"
	SeverityInfo     AlertSeverity = "info"
)

// SystemMetrics holds system-level data used by alert rules.
type SystemMetrics struct {
	DiskAvailableGB     float64
	ActiveFFmpegProcs   int
	DBLatencyMs         int
	TranscodeQueueDepth int
}

// AlertRule defines a named alerting condition.
type AlertRule struct {
	Name        string
	Description string
	Severity    AlertSeverity
	Evaluate    func(*qoe.DashboardMetrics, *SystemMetrics) bool
	Cooldown    time.Duration
}

// Alert is a fired alert instance.
type Alert struct {
	ID          string        `json:"id"`
	RuleName    string        `json:"rule_name"`
	Description string        `json:"description"`
	Severity    AlertSeverity `json:"severity"`
	FiredAt     time.Time     `json:"fired_at"`
	ResolvedAt  *time.Time    `json:"resolved_at,omitempty"`
	IsActive    bool          `json:"is_active"`
}

// Engine evaluates alert rules on a tick and manages active/resolved alerts.
type Engine struct {
	rules        []AlertRule
	activeAlerts map[string]*Alert // rule_name → active alert
	history      []*Alert
	mu           sync.RWMutex
	agg          *qoe.Aggregator
}

// NewEngine creates an alerting engine with the default rule set.
func NewEngine(agg *qoe.Aggregator) *Engine {
	e := &Engine{
		activeAlerts: make(map[string]*Alert),
		agg:          agg,
	}
	e.rules = defaultRules()
	return e
}

// Start begins evaluating rules every 10 seconds until ctx is cancelled.
func (e *Engine) Start(ctx context.Context, sysMetricsFn func() *SystemMetrics) {
	go func() {
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				e.evaluate(sysMetricsFn())
			}
		}
	}()
}

// ActiveAlerts returns a copy of currently active alerts.
func (e *Engine) ActiveAlerts() []*Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Alert, 0, len(e.activeAlerts))
	for _, a := range e.activeAlerts {
		cp := *a
		out = append(out, &cp)
	}
	return out
}

// History returns the most recent resolved and active alerts (up to 100).
func (e *Engine) History() []*Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Alert, len(e.history))
	for i, a := range e.history {
		cp := *a
		out[i] = &cp
	}
	return out
}

func (e *Engine) evaluate(sys *SystemMetrics) {
	m := e.agg.GetMetrics()
	if sys == nil {
		sys = &SystemMetrics{}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for _, rule := range e.rules {
		fired := rule.Evaluate(m, sys)
		existing, active := e.activeAlerts[rule.Name]

		if fired && !active {
			// Fire new alert
			a := &Alert{
				ID:          newID(),
				RuleName:    rule.Name,
				Description: rule.Description,
				Severity:    rule.Severity,
				FiredAt:     time.Now(),
				IsActive:    true,
			}
			e.activeAlerts[rule.Name] = a
			e.appendHistory(a)
			slog.Warn("alert fired",
				"rule", rule.Name,
				"severity", rule.Severity,
				"description", rule.Description,
			)
		} else if !fired && active {
			// Resolve existing alert
			now := time.Now()
			existing.ResolvedAt = &now
			existing.IsActive = false
			delete(e.activeAlerts, rule.Name)
			slog.Info("alert resolved", "rule", rule.Name)
		}
	}
}

func (e *Engine) appendHistory(a *Alert) {
	e.history = append(e.history, a)
	if len(e.history) > 100 {
		e.history = e.history[len(e.history)-100:]
	}
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func defaultRules() []AlertRule {
	return []AlertRule{
		{
			Name:        "high_rebuffer_rate",
			Description: "Rebuffer rate exceeds 5% with active sessions",
			Severity:    SeverityCritical,
			Cooldown:    5 * time.Minute,
			Evaluate: func(m *qoe.DashboardMetrics, _ *SystemMetrics) bool {
				return m.ActiveSessions > 0 && m.RebufferRate > 0.05
			},
		},
		{
			Name:        "disk_space_critical",
			Description: "Available disk space is below 1 GB",
			Severity:    SeverityCritical,
			Cooldown:    time.Minute,
			Evaluate: func(_ *qoe.DashboardMetrics, s *SystemMetrics) bool {
				return s.DiskAvailableGB > 0 && s.DiskAvailableGB < 1.0
			},
		},
		{
			Name:        "database_unhealthy",
			Description: "Database query latency exceeds 1000ms",
			Severity:    SeverityCritical,
			Cooldown:    time.Minute,
			Evaluate: func(_ *qoe.DashboardMetrics, s *SystemMetrics) bool {
				return s.DBLatencyMs > 1000
			},
		},
		{
			Name:        "slow_ttff",
			Description: "Median time to first frame exceeds 2000ms",
			Severity:    SeverityWarning,
			Cooldown:    5 * time.Minute,
			Evaluate: func(m *qoe.DashboardMetrics, _ *SystemMetrics) bool {
				return m.ActiveSessions > 0 && m.TTFFMedianMs > 2000
			},
		},
		{
			Name:        "transcode_queue_backed_up",
			Description: "Transcode queue depth exceeds 10 jobs",
			Severity:    SeverityWarning,
			Cooldown:    5 * time.Minute,
			Evaluate: func(_ *qoe.DashboardMetrics, s *SystemMetrics) bool {
				return s.TranscodeQueueDepth > 10
			},
		},
		{
			Name:        "disk_space_warning",
			Description: "Available disk space is below 5 GB",
			Severity:    SeverityWarning,
			Cooldown:    5 * time.Minute,
			Evaluate: func(_ *qoe.DashboardMetrics, s *SystemMetrics) bool {
				return s.DiskAvailableGB > 0 && s.DiskAvailableGB < 5.0
			},
		},
		{
			Name:        "high_quality_switches",
			Description: "Quality switch rate exceeds 3 per minute",
			Severity:    SeverityWarning,
			Cooldown:    5 * time.Minute,
			Evaluate: func(m *qoe.DashboardMetrics, _ *SystemMetrics) bool {
				return m.ActiveSessions > 0 && m.QualitySwitchesPerMin > 3
			},
		},
		{
			Name:        "no_active_sessions",
			Description: "No active playback sessions while live streams are broadcasting",
			Severity:    SeverityInfo,
			Cooldown:    10 * time.Minute,
			Evaluate: func(m *qoe.DashboardMetrics, _ *SystemMetrics) bool {
				return m.ActiveSessions == 0 && m.ActiveLiveStreams > 0
			},
		},
	}
}
