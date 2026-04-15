package watchdog

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"philos-video/internal/live"
	"philos-video/internal/metrics"
	jobrepo "philos-video/internal/storage/job"
)

// Watchdog monitors FFmpeg processes and stuck transcode jobs.
type Watchdog struct {
	liveMgr       *live.Manager
	jobRepo       jobrepo.Repository
	dataDir       string
	checkInterval time.Duration
}

// New creates a Watchdog.
func New(liveMgr *live.Manager, jobRepo jobrepo.Repository, dataDir string) *Watchdog {
	return &Watchdog{
		liveMgr:       liveMgr,
		jobRepo:       jobRepo,
		dataDir:       dataDir,
		checkInterval: 10 * time.Second,
	}
}

// Start begins the watchdog loop until ctx is cancelled.
func (w *Watchdog) Start(ctx context.Context) {
	go func() {
		tick := time.NewTicker(w.checkInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				w.checkLiveTranscoders()
				w.checkStuckJobs(ctx)
				w.checkOrphanedFFmpeg()
			}
		}
	}()
}

// checkLiveTranscoders verifies that each live FFmpeg process is still alive
// and producing segment files recently. Stalled or dead sessions are ended.
func (w *Watchdog) checkLiveTranscoders() {
	const segmentStaleness = 10 * time.Second

	pids := w.liveMgr.GetPIDs()
	for streamID, pid := range pids {
		// 1. Check process is alive (signal 0 = probe, no signal sent).
		if pid > 0 {
			if err := syscall.Kill(pid, 0); err != nil {
				slog.Warn("live transcoder process dead", "stream_id", streamID, "pid", pid)
				w.liveMgr.EndStream(streamID)
				continue
			}
		}

		// 2. Check that a segment file was written recently (process alive but stalled).
		newest := w.newestSegmentMtime(streamID)
		if !newest.IsZero() && time.Since(newest) > segmentStaleness {
			slog.Warn("live transcoder stalled — no new segment",
				"stream_id", streamID, "last_segment_age", time.Since(newest).Round(time.Second))
			w.liveMgr.EndStream(streamID)
		}
	}
}

// newestSegmentMtime returns the modification time of the most recently written
// .ts segment across all quality subdirs for a stream, or zero if none found.
func (w *Watchdog) newestSegmentMtime(streamID string) time.Time {
	streamDir := filepath.Join(w.dataDir, "live", streamID)
	var newest time.Time
	for _, quality := range []string{"720p", "480p", "360p"} {
		entries, err := os.ReadDir(filepath.Join(streamDir, quality))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".ts") {
				continue
			}
			if info, err := e.Info(); err == nil {
				if info.ModTime().After(newest) {
					newest = info.ModTime()
				}
			}
		}
	}
	return newest
}

// checkStuckJobs finds transcode jobs that have been running for >30 minutes
// and resets them to queued so they can be retried.
func (w *Watchdog) checkStuckJobs(ctx context.Context) {
	stuck, err := w.jobRepo.FindStuck(ctx, 30*time.Minute)
	if err != nil {
		slog.Warn("watchdog: finding stuck jobs", "err", err)
		return
	}
	for _, job := range stuck {
		slog.Warn("resetting stuck job", "job_id", job.ID, "video_id", job.VideoID)
		if err := w.jobRepo.ResetToQueued(ctx, job.ID); err != nil {
			slog.Warn("resetting stuck job", "job_id", job.ID, "err", err)
			continue
		}
		metrics.TranscodeJobsTotal.WithLabelValues("reset_stuck").Inc()
	}
}

// checkOrphanedFFmpeg tracks the total FFmpeg process count and SIGTERMs any
// live-ingest FFmpeg process (identified by "-i pipe:0") that is no longer
// tracked by the live manager. VOD transcoding processes never use pipe:0 and
// are therefore never touched here.
func (w *Watchdog) checkOrphanedFFmpeg() {
	// Total FFmpeg count for the gauge (all purposes).
	if all, err := exec.Command("pgrep", "-f", "ffmpeg").Output(); err == nil {
		metrics.FFmpegProcesses.Set(float64(len(bytes.Fields(all))))
	} else {
		metrics.FFmpegProcesses.Set(0)
	}

	// Only inspect live-ingest processes (use stdin pipe).
	out, err := exec.Command("pgrep", "-f", "ffmpeg.*pipe:0").Output()
	if err != nil {
		// pgrep exits 1 when no match — not an error.
		return
	}

	trackedPIDs := w.liveMgr.GetPIDs()
	tracked := make(map[int]bool, len(trackedPIDs))
	for _, pid := range trackedPIDs {
		tracked[pid] = true
	}

	for _, line := range bytes.Fields(out) {
		pid, err := strconv.Atoi(strings.TrimSpace(string(line)))
		if err != nil || pid <= 0 {
			continue
		}
		if !tracked[pid] {
			slog.Warn("orphaned live FFmpeg process found, sending SIGTERM", "pid", pid)
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
	}
}
