package health

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"os/exec"
)

// HealthChecker runs liveness and readiness probes.
type HealthChecker struct {
	db        *sql.DB
	dataDir   string
	rtmpPort  int
	startTime time.Time
}

// CheckResult holds the outcome of a single health check.
type CheckResult struct {
	Status    string `json:"status"`
	LatencyMs int    `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// NewHealthChecker creates a HealthChecker.
func NewHealthChecker(db *sql.DB, dataDir string, rtmpPort int) *HealthChecker {
	return &HealthChecker{
		db:        db,
		dataDir:   dataDir,
		rtmpPort:  rtmpPort,
		startTime: time.Now(),
	}
}

// Liveness returns a simple uptime response. Always succeeds.
func (h *HealthChecker) Liveness() map[string]any {
	return map[string]any{
		"status":         "ok",
		"uptime_seconds": int(time.Since(h.startTime).Seconds()),
	}
}

// Readiness runs all dependency checks and returns (results, allHealthy).
func (h *HealthChecker) Readiness(ctx context.Context) (map[string]CheckResult, bool) {
	results := map[string]CheckResult{
		"postgres":  h.checkPostgres(ctx),
		"ffmpeg":    h.checkFFmpeg(ctx),
		"disk":      h.checkDiskSpace(),
		"data_dir":  h.checkDataDir(),
		"rtmp_port": h.checkRTMPPort(),
	}

	allHealthy := true
	for _, r := range results {
		if r.Status != "ok" {
			allHealthy = false
			break
		}
	}
	return results, allHealthy
}

func (h *HealthChecker) checkPostgres(ctx context.Context) CheckResult {
	start := time.Now()
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(pingCtx); err != nil {
		return CheckResult{Status: "error", LatencyMs: ms(start), Error: err.Error()}
	}
	return CheckResult{Status: "ok", LatencyMs: ms(start)}
}

func (h *HealthChecker) checkFFmpeg(ctx context.Context) CheckResult {
	start := time.Now()
	execCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := exec.CommandContext(execCtx, "ffmpeg", "-version").Run(); err != nil {
		return CheckResult{Status: "error", LatencyMs: ms(start), Error: "ffmpeg not available"}
	}
	return CheckResult{Status: "ok", LatencyMs: ms(start)}
}

func (h *HealthChecker) checkDiskSpace() CheckResult {
	start := time.Now()
	var stat syscall.Statfs_t
	if err := syscall.Statfs(h.dataDir, &stat); err != nil {
		return CheckResult{Status: "error", LatencyMs: ms(start), Error: err.Error()}
	}
	available := stat.Bavail * uint64(stat.Bsize)
	const GB = 1 << 30
	switch {
	case available < GB:
		return CheckResult{Status: "error", LatencyMs: ms(start), Error: fmt.Sprintf("critical: only %.1f GB available", float64(available)/GB)}
	case available < 5*GB:
		return CheckResult{Status: "warn", LatencyMs: ms(start), Error: fmt.Sprintf("low disk: %.1f GB available", float64(available)/GB)}
	}
	return CheckResult{Status: "ok", LatencyMs: ms(start)}
}

func (h *HealthChecker) checkDataDir() CheckResult {
	start := time.Now()
	probe := filepath.Join(h.dataDir, ".health_check")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return CheckResult{Status: "error", LatencyMs: ms(start), Error: err.Error()}
	}
	if err := os.Remove(probe); err != nil {
		return CheckResult{Status: "warn", LatencyMs: ms(start), Error: "could not remove probe file"}
	}
	return CheckResult{Status: "ok", LatencyMs: ms(start)}
}

func (h *HealthChecker) checkRTMPPort() CheckResult {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", h.rtmpPort), time.Second)
	if err != nil {
		return CheckResult{Status: "error", LatencyMs: ms(start), Error: "RTMP port unreachable"}
	}
	conn.Close()
	return CheckResult{Status: "ok", LatencyMs: ms(start)}
}

func ms(start time.Time) int {
	return int(time.Since(start).Milliseconds())
}
