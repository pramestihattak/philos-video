package metrics

import (
	"context"
	"database/sql"
	"io/fs"
	"log/slog"
	"path/filepath"
	"syscall"
	"time"
)

// StartSystemCollector starts a background goroutine that updates system metrics every 15s.
func StartSystemCollector(ctx context.Context, dataDir string, db *sql.DB) {
	startTime := time.Now()
	go func() {
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		collect(startTime, dataDir, db)
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				collect(startTime, dataDir, db)
			}
		}
	}()
}

func collect(startTime time.Time, dataDir string, db *sql.DB) {
	ServerUptimeSeconds.Set(time.Since(startTime).Seconds())

	// Disk usage per directory
	for _, dir := range []string{"chunks", "raw", "hls", "live"} {
		full := filepath.Join(dataDir, dir)
		var total int64
		_ = filepath.WalkDir(full, func(_ string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				if info, err := d.Info(); err == nil {
					total += info.Size()
				}
			}
			return nil
		})
		StorageBytesUsed.WithLabelValues(dir).Set(float64(total))
	}

	// Available disk space on the data volume
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err == nil {
		available := stat.Bavail * uint64(stat.Bsize)
		StorageBytesAvailable.Set(float64(available))
	} else {
		slog.Warn("syscall.Statfs failed", "err", err)
	}

	// Database connection pool stats and queue depth
	if db != nil {
		DatabaseConnectionsActive.Set(float64(db.Stats().InUse))

		var queueDepth int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM transcode_jobs WHERE status='queued'`).Scan(&queueDepth); err == nil {
			TranscodeQueueDepth.Set(float64(queueDepth))
		}
	}
}
