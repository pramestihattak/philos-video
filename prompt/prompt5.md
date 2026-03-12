# Claude Code Prompt: Video Streaming Platform — Phase 5

## Context

I'm building a video streaming platform from scratch. I've completed:

- **Phase 1:** FFmpeg transcoding into multi-quality HLS, hls.js player with ABR
- **Phase 2:** Chunked upload API, PostgreSQL, background transcode workers, web UI
- **Phase 3:** Playback sessions with signed JWT URLs, client telemetry pipeline, QoE aggregator with real-time SSE dashboard
- **Phase 4:** RTMP ingest server, real-time live transcoding, live HLS serving, live-to-VOD conversion, Go Live UI

The platform is functionally complete. Now I'm adding production-grade observability and reliability: Prometheus metrics, structured log correlation, health checks, graceful shutdown, and an alerting system.

## What To Build

### Overview

Six capabilities to make this production-ready:

1. **Prometheus Metrics** — Instrument every layer with counters, gauges, histograms. Expose /metrics endpoint.
2. **Structured Log Correlation** — Request IDs and trace context flowing through every log line. Correlation between upload → transcode → serve → playback.
3. **Health Checks** — Liveness and readiness endpoints. Dependency health (PostgreSQL, FFmpeg, disk space).
4. **Graceful Shutdown** — Clean shutdown sequence: stop accepting new streams, drain active connections, wait for in-flight transcodes, close database.
5. **Process Watchdog** — Monitor FFmpeg processes, detect stuck transcodes, auto-recover crashed live streams.
6. **Alerting Dashboard** — Alert rules engine that evaluates metrics every 10 seconds, fires alerts displayed on the QoE dashboard, and logs them.

---

### 1. Prometheus Metrics

Add `github.com/prometheus/client_golang` for metrics instrumentation.

#### Metric Definitions

```go
// internal/metrics/metrics.go

package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// ══════════════ HTTP Server Metrics ══════════════

var (
    HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_http_requests_total",
        Help: "Total HTTP requests by method, path pattern, and status code",
    }, []string{"method", "path_pattern", "status_code"})

    HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "video_http_request_duration_seconds",
        Help:    "HTTP request duration in seconds",
        Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
    }, []string{"method", "path_pattern"})

    HTTPResponseBytes = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_http_response_bytes_total",
        Help: "Total bytes sent in HTTP responses",
    }, []string{"path_pattern"})
)

// ══════════════ Upload Metrics ══════════════

var (
    UploadsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_uploads_total",
        Help: "Total uploads by status (started, completed, failed)",
    }, []string{"status"})

    UploadBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "video_upload_bytes_total",
        Help: "Total bytes uploaded",
    })

    UploadChunkDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "video_upload_chunk_duration_seconds",
        Help:    "Time to receive and store a single upload chunk",
        Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
    })

    ActiveUploads = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "video_active_uploads",
        Help: "Number of uploads currently in progress",
    })
)

// ══════════════ Transcoding Metrics ══════════════

var (
    TranscodeQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "video_transcode_queue_depth",
        Help: "Number of transcode jobs waiting in queue",
    })

    TranscodeJobsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_transcode_jobs_total",
        Help: "Total transcode jobs by status",
    }, []string{"status"}) // started, completed, failed

    TranscodeJobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "video_transcode_job_duration_seconds",
        Help:    "Total time to transcode a video",
        Buckets: []float64{10, 30, 60, 120, 300, 600, 1200, 1800, 3600},
    }, []string{"source_resolution"})

    TranscodeEncodeDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "video_transcode_encode_duration_seconds",
        Help:    "Time to encode a single quality level",
        Buckets: []float64{5, 15, 30, 60, 120, 300, 600, 1200},
    }, []string{"resolution", "codec"})

    TranscodeSpeedRatio = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "video_transcode_speed_ratio",
        Help: "Encoding speed as ratio of real-time (>1 = faster than real-time)",
    }, []string{"stream_id", "resolution"})

    TranscodeActiveWorkers = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "video_transcode_active_workers",
        Help: "Number of transcode workers currently processing a job",
    })

    FFmpegProcesses = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "video_ffmpeg_processes",
        Help: "Number of FFmpeg processes currently running",
    })
)

// ══════════════ Live Streaming Metrics ══════════════

var (
    LiveStreamsActive = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "video_live_streams_active",
        Help: "Number of currently active live streams",
    })

    LiveStreamsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_live_streams_total",
        Help: "Total live streams by end reason",
    }, []string{"end_reason"}) // disconnected, manual_end, error, replaced

    LiveStreamDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "video_live_stream_duration_seconds",
        Help:    "Duration of live streams",
        Buckets: []float64{60, 300, 600, 1800, 3600, 7200, 14400},
    })

    LiveIngestBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "video_live_ingest_bytes_total",
        Help: "Total bytes received from RTMP ingest",
    })

    LiveViewersActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "video_live_viewers_active",
        Help: "Active viewers per live stream",
    }, []string{"stream_id"})

    RTMPConnectionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_rtmp_connections_total",
        Help: "Total RTMP connections by result",
    }, []string{"result"}) // accepted, rejected_auth, rejected_duplicate, error
)

// ══════════════ Delivery Metrics ══════════════

var (
    SegmentRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_segment_requests_total",
        Help: "Total segment requests by type and quality",
    }, []string{"type", "quality"}) // type: vod, live; quality: 720p, 480p, 360p

    SegmentRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "video_segment_request_duration_seconds",
        Help:    "Time to serve a segment request",
        Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
    }, []string{"type"})

    SegmentBytesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_segment_bytes_total",
        Help: "Total bytes served for segments",
    }, []string{"type", "quality"})

    ManifestRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_manifest_requests_total",
        Help: "Total manifest requests by type",
    }, []string{"type"}) // master, variant

    TokenValidationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_token_validations_total",
        Help: "Total JWT token validations by result",
    }, []string{"result"}) // valid, expired, invalid, missing
)

// ══════════════ Playback QoE Metrics (from client telemetry) ══════════════

var (
    PlaybackSessionsActive = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "video_playback_sessions_active",
        Help: "Number of active playback sessions",
    })

    PlaybackSessionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_playback_sessions_total",
        Help: "Total playback sessions by type",
    }, []string{"type"}) // vod, live

    PlaybackTTFFSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "video_playback_ttff_seconds",
        Help:    "Time to first frame in seconds",
        Buckets: []float64{0.25, 0.5, 0.75, 1.0, 1.5, 2.0, 3.0, 5.0, 10.0},
    })

    PlaybackRebufferTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "video_playback_rebuffer_events_total",
        Help: "Total rebuffer events across all sessions",
    })

    PlaybackRebufferDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "video_playback_rebuffer_duration_seconds",
        Help:    "Duration of individual rebuffer events",
        Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
    })

    PlaybackQualitySwitchesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_playback_quality_switches_total",
        Help: "Total quality switches by direction",
    }, []string{"direction"}) // up, down

    PlaybackBitrateKbps = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "video_playback_bitrate_kbps",
        Help:    "Bitrate being consumed by viewers",
        Buckets: []float64{300, 600, 1000, 1500, 2000, 2500, 3500, 5000},
    })

    PlaybackErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_playback_errors_total",
        Help: "Total playback errors by error code",
    }, []string{"error_code"})

    TelemetryEventsReceived = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "video_telemetry_events_received_total",
        Help: "Total telemetry events received by type",
    }, []string{"event_type"})
)

// ══════════════ System Metrics ══════════════

var (
    StorageBytesUsed = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "video_storage_bytes_used",
        Help: "Disk space used by directory",
    }, []string{"directory"}) // hls, live, raw, chunks

    StorageBytesAvailable = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "video_storage_bytes_available",
        Help: "Available disk space on the data partition",
    })

    DatabaseConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "video_database_connections_active",
        Help: "Active PostgreSQL connections",
    })

    DatabaseQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "video_database_query_duration_seconds",
        Help:    "Database query duration",
        Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
    }, []string{"operation"}) // select, insert, update

    ServerUptimeSeconds = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "video_server_uptime_seconds",
        Help: "Server uptime in seconds",
    })
)
```

#### Instrumenting Existing Code

Add metric recording to existing handlers and services. Don't rewrite them — just add the recording calls.

**HTTP Middleware for request metrics:**

```go
// internal/middleware/metrics.go

func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Wrap response writer to capture status code and bytes written
        wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

        next.ServeHTTP(wrapped, r)

        duration := time.Since(start).Seconds()
        pattern := categorizePathPattern(r.URL.Path)
        status := strconv.Itoa(wrapped.statusCode)

        metrics.HTTPRequestsTotal.WithLabelValues(r.Method, pattern, status).Inc()
        metrics.HTTPRequestDuration.WithLabelValues(r.Method, pattern).Observe(duration)
        metrics.HTTPResponseBytes.WithLabelValues(pattern).Add(float64(wrapped.bytesWritten))
    })
}

type responseWriter struct {
    http.ResponseWriter
    statusCode   int
    bytesWritten int64
}

func (w *responseWriter) WriteHeader(code int) {
    w.statusCode = code
    w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
    n, err := w.ResponseWriter.Write(b)
    w.bytesWritten += int64(n)
    return n, err
}

// Categorize URL paths into patterns to avoid high-cardinality labels
// CRITICAL: Never use raw URLs as label values — this explodes Prometheus storage
func categorizePathPattern(path string) string {
    switch {
    case path == "/":
        return "/"
    case path == "/metrics":
        return "/metrics"
    case path == "/health", path == "/health/ready":
        return "/health"
    case path == "/dashboard":
        return "/dashboard"
    case strings.HasPrefix(path, "/api/v1/uploads") && strings.Contains(path, "/chunks/"):
        return "/api/v1/uploads/*/chunks/*"
    case strings.HasPrefix(path, "/api/v1/uploads"):
        return "/api/v1/uploads/*"
    case strings.HasPrefix(path, "/api/v1/videos") && strings.Contains(path, "/sessions"):
        return "/api/v1/videos/*/sessions"
    case strings.HasPrefix(path, "/api/v1/videos"):
        return "/api/v1/videos/*"
    case strings.HasPrefix(path, "/api/v1/sessions") && strings.Contains(path, "/events"):
        return "/api/v1/sessions/*/events"
    case strings.HasPrefix(path, "/api/v1/live"):
        return "/api/v1/live/*"
    case strings.HasPrefix(path, "/api/v1/dashboard"):
        return "/api/v1/dashboard/*"
    case strings.HasPrefix(path, "/videos/") && strings.HasSuffix(path, ".m3u8"):
        return "/videos/*/manifest"
    case strings.HasPrefix(path, "/videos/") && (strings.HasSuffix(path, ".m4s") || strings.HasSuffix(path, ".mp4")):
        return "/videos/*/segment"
    case strings.HasPrefix(path, "/live/") && strings.HasSuffix(path, ".m3u8"):
        return "/live/*/manifest"
    case strings.HasPrefix(path, "/live/") && (strings.HasSuffix(path, ".m4s") || strings.HasSuffix(path, ".mp4")):
        return "/live/*/segment"
    default:
        return "/other"
    }
}
```

**Instrument the upload handler (example):**

```go
func (h *UploadHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
    timer := prometheus.NewTimer(metrics.UploadChunkDuration)
    defer timer.ObserveDuration()

    // ... existing chunk upload logic ...

    metrics.UploadBytesTotal.Add(float64(bytesWritten))
}
```

**Instrument the transcode worker:**

```go
func (w *Worker) processJob(ctx context.Context, job *models.TranscodeJob) {
    metrics.TranscodeActiveWorkers.Inc()
    defer metrics.TranscodeActiveWorkers.Dec()

    metrics.TranscodeJobsTotal.WithLabelValues("started").Inc()
    jobStart := time.Now()

    // ... existing transcode logic ...
    // When encoding each quality:
    encodeStart := time.Now()
    err := w.transcoder.Encode(ctx, profile, inputPath, outputPath)
    metrics.TranscodeEncodeDuration.WithLabelValues(
        profile.Name, "h264",
    ).Observe(time.Since(encodeStart).Seconds())

    if err != nil {
        metrics.TranscodeJobsTotal.WithLabelValues("failed").Inc()
        return
    }

    metrics.TranscodeJobsTotal.WithLabelValues("completed").Inc()
    metrics.TranscodeJobDuration.WithLabelValues(
        job.SourceResolution,
    ).Observe(time.Since(jobStart).Seconds())
}
```

**Instrument the telemetry handler (connect client events to Prometheus):**

```go
func (h *TelemetryHandler) PostEvents(w http.ResponseWriter, r *http.Request) {
    // ... existing decode + batch insert ...

    // Record each event type in Prometheus
    for _, event := range req.Events {
        metrics.TelemetryEventsReceived.WithLabelValues(event.EventType).Inc()

        switch event.EventType {
        case "playback_start":
            if event.DownloadTimeMs > 0 {
                metrics.PlaybackTTFFSeconds.Observe(float64(event.DownloadTimeMs) / 1000.0)
            }
        case "rebuffer_start":
            metrics.PlaybackRebufferTotal.Inc()
        case "rebuffer_end":
            if event.RebufferDurationMs > 0 {
                metrics.PlaybackRebufferDuration.Observe(float64(event.RebufferDurationMs) / 1000.0)
            }
        case "quality_change":
            direction := "down"
            if qualityRank(event.QualityTo) > qualityRank(event.QualityFrom) {
                direction = "up"
            }
            metrics.PlaybackQualitySwitchesTotal.WithLabelValues(direction).Inc()
        case "segment_downloaded":
            if event.ThroughputBps > 0 {
                metrics.PlaybackBitrateKbps.Observe(float64(event.ThroughputBps) / 1000.0)
            }
        case "error":
            code := event.ErrorCode
            if code == "" { code = "unknown" }
            metrics.PlaybackErrorsTotal.WithLabelValues(code).Inc()
        }
    }
}

func qualityRank(q string) int {
    switch q {
    case "360p": return 1
    case "480p": return 2
    case "720p": return 3
    case "1080p": return 4
    default: return 0
    }
}
```

**System metrics collector (background goroutine):**

```go
// internal/metrics/system_collector.go

func StartSystemCollector(ctx context.Context, dataDir string, db *sql.DB) {
    ticker := time.NewTicker(15 * time.Second)
    startTime := time.Now()
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Uptime
            ServerUptimeSeconds.Set(time.Since(startTime).Seconds())

            // Disk usage per directory
            for _, dir := range []string{"hls", "live", "raw", "chunks"} {
                path := filepath.Join(dataDir, dir)
                size := dirSize(path)
                StorageBytesUsed.WithLabelValues(dir).Set(float64(size))
            }

            // Available disk space
            var stat syscall.Statfs_t
            if err := syscall.Statfs(dataDir, &stat); err == nil {
                available := stat.Bavail * uint64(stat.Bsize)
                StorageBytesAvailable.Set(float64(available))
            }

            // Database connection pool stats
            stats := db.Stats()
            DatabaseConnectionsActive.Set(float64(stats.InUse))
        }
    }
}

func dirSize(path string) int64 {
    var size int64
    filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return nil
        }
        size += info.Size()
        return nil
    })
    return size
}
```

#### Expose /metrics Endpoint

```go
import "github.com/prometheus/client_golang/prometheus/promhttp"

// In route registration:
mux.Handle("GET /metrics", promhttp.Handler())
```

---

### 2. Structured Log Correlation

Add request IDs that flow through every log line for a given operation.

#### Request ID Middleware

```go
// internal/middleware/request_id.go

type contextKey string
const RequestIDKey contextKey = "request_id"

func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = generateShortID() // 12-char random string
        }
        
        w.Header().Set("X-Request-ID", requestID)
        ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func RequestIDFromContext(ctx context.Context) string {
    if id, ok := ctx.Value(RequestIDKey).(string); ok {
        return id
    }
    return ""
}
```

#### Context-Aware Logger

```go
// internal/logging/logger.go

// Create a logger with context fields already attached
func FromContext(ctx context.Context) *slog.Logger {
    logger := slog.Default()
    
    if reqID := middleware.RequestIDFromContext(ctx); reqID != "" {
        logger = logger.With(slog.String("request_id", reqID))
    }
    if videoID := VideoIDFromContext(ctx); videoID != "" {
        logger = logger.With(slog.String("video_id", videoID))
    }
    if sessionID := SessionIDFromContext(ctx); sessionID != "" {
        logger = logger.With(slog.String("session_id", sessionID))
    }
    if streamID := StreamIDFromContext(ctx); streamID != "" {
        logger = logger.With(slog.String("stream_id", streamID))
    }
    if jobID := JobIDFromContext(ctx); jobID != "" {
        logger = logger.With(slog.String("job_id", jobID))
    }
    
    return logger
}

// Context setters used by handlers when they extract IDs from the request
func WithVideoID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, contextKey("video_id"), id)
}
func VideoIDFromContext(ctx context.Context) string {
    if id, ok := ctx.Value(contextKey("video_id")).(string); ok { return id }
    return ""
}
// ... same pattern for session_id, stream_id, job_id
```

#### Structured Log Format

Configure slog to output JSON in production:

```go
// internal/logging/setup.go

func Setup(level string, format string) {
    var handler slog.Handler
    
    opts := &slog.HandlerOptions{
        Level: parseLevel(level),
        // Add source file/line for error logs
        AddSource: true,
    }
    
    switch format {
    case "json":
        handler = slog.NewJSONHandler(os.Stdout, opts)
    default:
        handler = slog.NewTextHandler(os.Stdout, opts)
    }
    
    slog.SetDefault(slog.New(handler))
}

func parseLevel(s string) slog.Level {
    switch strings.ToLower(s) {
    case "debug": return slog.LevelDebug
    case "warn": return slog.LevelWarn
    case "error": return slog.LevelError
    default: return slog.LevelInfo
    }
}
```

#### Update Existing Code to Use Context Logger

Replace all `slog.Info(...)` calls with `logging.FromContext(ctx).Info(...)`:

```go
// BEFORE (existing code):
slog.Info("upload_chunk_received",
    slog.String("upload_id", uploadID),
    slog.Int("chunk_number", chunkNum),
)

// AFTER:
logging.FromContext(ctx).Info("upload_chunk_received",
    slog.Int("chunk_number", chunkNum),
    slog.Int64("bytes", bytesWritten),
)
// request_id, video_id etc. are automatically included from context
```

#### Correlation Example

After instrumentation, a full upload-to-playback trace looks like this in logs:

```json
{"time":"...","level":"INFO","msg":"upload_initialized","request_id":"req_abc","video_id":"vid_123","chunks":103}
{"time":"...","level":"INFO","msg":"upload_chunk_received","request_id":"req_def","video_id":"vid_123","chunk":1,"bytes":5242880}
{"time":"...","level":"INFO","msg":"upload_chunk_received","request_id":"req_ghi","video_id":"vid_123","chunk":2,"bytes":5242880}
...
{"time":"...","level":"INFO","msg":"upload_complete","request_id":"req_xyz","video_id":"vid_123","total_bytes":541065216}
{"time":"...","level":"INFO","msg":"transcode_started","video_id":"vid_123","job_id":"job_456","source_resolution":"1920x1080"}
{"time":"...","level":"INFO","msg":"encode_complete","video_id":"vid_123","job_id":"job_456","resolution":"720p","duration_s":45.2,"speed_ratio":8.3}
{"time":"...","level":"INFO","msg":"encode_complete","video_id":"vid_123","job_id":"job_456","resolution":"480p","duration_s":32.1,"speed_ratio":11.7}
{"time":"...","level":"INFO","msg":"transcode_complete","video_id":"vid_123","job_id":"job_456","total_duration_s":92.5}
{"time":"...","level":"INFO","msg":"session_created","session_id":"sess_789","video_id":"vid_123","device":"desktop"}
{"time":"...","level":"INFO","msg":"events_received","session_id":"sess_789","video_id":"vid_123","count":5}
```

You can now grep `video_id:"vid_123"` to see the entire lifecycle of one video.

---

### 3. Health Checks

#### Endpoints

**GET /health** — Liveness probe (is the server running?)
```json
// Always returns 200 if the server is responding
{ "status": "ok", "uptime_seconds": 3600 }
```

**GET /health/ready** — Readiness probe (is the server ready to handle traffic?)
```json
// Returns 200 only if all dependencies are healthy
{
  "status": "ready",
  "checks": {
    "postgresql": { "status": "healthy", "latency_ms": 2 },
    "ffmpeg": { "status": "healthy", "version": "6.1" },
    "disk_space": { "status": "healthy", "available_gb": 45.2, "threshold_gb": 1.0 },
    "data_directory": { "status": "healthy", "writable": true },
    "rtmp_server": { "status": "healthy", "port": 1935 }
  }
}

// Returns 503 if any critical check fails
{
  "status": "not_ready",
  "checks": {
    "postgresql": { "status": "unhealthy", "error": "connection refused" },
    "ffmpeg": { "status": "healthy", "version": "6.1" },
    "disk_space": { "status": "warning", "available_gb": 0.8, "threshold_gb": 1.0 },
    "data_directory": { "status": "healthy", "writable": true },
    "rtmp_server": { "status": "healthy", "port": 1935 }
  }
}
```

#### Implementation

```go
// internal/health/checker.go

type HealthChecker struct {
    db       *sql.DB
    dataDir  string
    rtmpPort int
    startTime time.Time
}

type CheckResult struct {
    Status   string `json:"status"`  // healthy, unhealthy, warning
    LatencyMs int   `json:"latency_ms,omitempty"`
    Error    string `json:"error,omitempty"`
    // Additional fields per check type
}

func (h *HealthChecker) CheckAll(ctx context.Context) (map[string]CheckResult, bool) {
    checks := make(map[string]CheckResult)
    allHealthy := true

    // PostgreSQL
    checks["postgresql"] = h.checkPostgres(ctx)
    if checks["postgresql"].Status == "unhealthy" { allHealthy = false }

    // FFmpeg
    checks["ffmpeg"] = h.checkFFmpeg(ctx)
    if checks["ffmpeg"].Status == "unhealthy" { allHealthy = false }

    // Disk space
    checks["disk_space"] = h.checkDiskSpace()
    if checks["disk_space"].Status == "unhealthy" { allHealthy = false }

    // Data directory writable
    checks["data_directory"] = h.checkDataDir()
    if checks["data_directory"].Status == "unhealthy" { allHealthy = false }

    // RTMP server
    checks["rtmp_server"] = h.checkRTMPPort()

    return checks, allHealthy
}

func (h *HealthChecker) checkPostgres(ctx context.Context) CheckResult {
    start := time.Now()
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    
    err := h.db.PingContext(ctx)
    latency := int(time.Since(start).Milliseconds())
    
    if err != nil {
        return CheckResult{Status: "unhealthy", Error: err.Error(), LatencyMs: latency}
    }
    return CheckResult{Status: "healthy", LatencyMs: latency}
}

func (h *HealthChecker) checkFFmpeg(ctx context.Context) CheckResult {
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    
    cmd := exec.CommandContext(ctx, "ffmpeg", "-version")
    output, err := cmd.Output()
    if err != nil {
        return CheckResult{Status: "unhealthy", Error: "ffmpeg not found: " + err.Error()}
    }
    
    // Extract version from first line
    version := strings.Split(strings.Split(string(output), "\n")[0], " ")[2]
    return CheckResult{Status: "healthy", Error: version} // reuse Error field for version display
}

func (h *HealthChecker) checkDiskSpace() CheckResult {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(h.dataDir, &stat); err != nil {
        return CheckResult{Status: "unhealthy", Error: err.Error()}
    }
    
    availableGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
    thresholdGB := 1.0
    
    if availableGB < thresholdGB {
        return CheckResult{Status: "unhealthy", Error: fmt.Sprintf("%.1f GB available (threshold: %.1f GB)", availableGB, thresholdGB)}
    }
    if availableGB < thresholdGB*5 {
        return CheckResult{Status: "warning", Error: fmt.Sprintf("%.1f GB available", availableGB)}
    }
    return CheckResult{Status: "healthy", Error: fmt.Sprintf("%.1f GB available", availableGB)}
}

func (h *HealthChecker) checkDataDir() CheckResult {
    testFile := filepath.Join(h.dataDir, ".health_check")
    err := os.WriteFile(testFile, []byte("ok"), 0644)
    if err != nil {
        return CheckResult{Status: "unhealthy", Error: "not writable: " + err.Error()}
    }
    os.Remove(testFile)
    return CheckResult{Status: "healthy"}
}

func (h *HealthChecker) checkRTMPPort() CheckResult {
    conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", h.rtmpPort), 1*time.Second)
    if err != nil {
        return CheckResult{Status: "unhealthy", Error: "RTMP port not listening"}
    }
    conn.Close()
    return CheckResult{Status: "healthy"}
}
```

---

### 4. Graceful Shutdown

The server must shut down cleanly when receiving SIGTERM or SIGINT.

```go
// cmd/server/main.go — updated main function

func main() {
    // ... setup code ...
    
    // Create root context that cancels on shutdown signal
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()
    
    // Start all components with this context
    go rtmpServer.Start(ctx)
    go aggregator.Run(ctx)
    go metrics.StartSystemCollector(ctx, cfg.DataDir, db)
    go watchdog.Start(ctx)
    for i := 0; i < cfg.WorkerCount; i++ {
        go vodWorker.Run(ctx, jobChan)
    }
    
    // HTTP server with graceful shutdown
    httpServer := &http.Server{
        Addr:    fmt.Sprintf(":%d", cfg.Port),
        Handler: mux,
    }
    
    // Start HTTP server in goroutine
    go func() {
        slog.Info("http_server_started", slog.Int("port", cfg.Port))
        if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
            slog.Error("http_server_error", slog.String("error", err.Error()))
        }
    }()
    
    // Wait for shutdown signal
    <-ctx.Done()
    slog.Info("shutdown_initiated", slog.String("signal", "received"))
    
    // ── SHUTDOWN SEQUENCE ──
    
    // 1. Stop accepting new RTMP connections (context canceled above)
    slog.Info("shutdown_step", slog.String("step", "rtmp_stopped"))
    
    // 2. End all active live streams gracefully
    activeStreams := liveService.GetAllActive(context.Background())
    for _, stream := range activeStreams {
        slog.Info("shutdown_ending_stream", slog.String("stream_id", stream.ID))
        liveService.EndStream(context.Background(), stream.ID, "server_shutdown")
    }
    slog.Info("shutdown_step", slog.String("step", "live_streams_ended"),
        slog.Int("count", len(activeStreams)))
    
    // 3. Stop HTTP server with timeout (drain active connections)
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer shutdownCancel()
    
    if err := httpServer.Shutdown(shutdownCtx); err != nil {
        slog.Error("http_shutdown_error", slog.String("error", err.Error()))
    }
    slog.Info("shutdown_step", slog.String("step", "http_drained"))
    
    // 4. Wait for in-flight transcode jobs to finish (with timeout)
    slog.Info("shutdown_step", slog.String("step", "waiting_for_transcodes"))
    close(jobChan) // signal workers to stop after current job
    
    workerDone := make(chan struct{})
    go func() {
        vodWorker.WaitAll() // blocks until all workers finish current job
        close(workerDone)
    }()
    
    select {
    case <-workerDone:
        slog.Info("shutdown_step", slog.String("step", "transcodes_complete"))
    case <-time.After(60 * time.Second):
        slog.Warn("shutdown_step", slog.String("step", "transcodes_timeout"))
    }
    
    // 5. Close database connection
    db.Close()
    slog.Info("shutdown_complete")
}
```

Update the worker to support graceful drain:

```go
// internal/worker/transcode_worker.go

type WorkerPool struct {
    wg sync.WaitGroup
    // ... existing fields
}

func (wp *WorkerPool) Run(ctx context.Context, jobChan <-chan string) {
    wp.wg.Add(1)
    defer wp.wg.Done()
    
    for jobID := range jobChan {  // exits when channel is closed
        select {
        case <-ctx.Done():
            slog.Info("worker_shutdown", slog.String("reason", "context_canceled"))
            return
        default:
            wp.processJob(ctx, jobID)
        }
    }
}

func (wp *WorkerPool) WaitAll() {
    wp.wg.Wait()
}
```

---

### 5. Process Watchdog

Monitor FFmpeg processes and detect problems.

```go
// internal/watchdog/watchdog.go

type Watchdog struct {
    liveService   *service.LiveService
    jobRepo       repository.JobRepo
    checkInterval time.Duration  // 10 seconds
    logger        *slog.Logger
}

func (w *Watchdog) Start(ctx context.Context) {
    ticker := time.NewTicker(w.checkInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            w.checkLiveTranscoders(ctx)
            w.checkStuckJobs(ctx)
            w.checkOrphanedProcesses(ctx)
        }
    }
}

// Check that all "live" streams still have a running FFmpeg process
func (w *Watchdog) checkLiveTranscoders(ctx context.Context) {
    streams, _ := w.liveService.GetAllActive(ctx)
    
    for _, stream := range streams {
        if stream.FFmpegPID == 0 {
            continue
        }
        
        // Check if process is still running
        process, err := os.FindProcess(stream.FFmpegPID)
        if err != nil {
            w.logger.Error("watchdog_process_not_found",
                slog.String("stream_id", stream.ID),
                slog.Int("pid", stream.FFmpegPID),
            )
            w.handleCrashedStream(ctx, stream)
            continue
        }
        
        // On Unix, FindProcess always succeeds. Send signal 0 to check if alive.
        err = process.Signal(syscall.Signal(0))
        if err != nil {
            w.logger.Error("watchdog_process_dead",
                slog.String("stream_id", stream.ID),
                slog.Int("pid", stream.FFmpegPID),
                slog.String("error", err.Error()),
            )
            w.handleCrashedStream(ctx, stream)
            continue
        }
        
        // Check if new segments are being produced (stall detection)
        latestSegment := w.findLatestSegment(stream.HLSPath)
        if latestSegment != "" {
            info, _ := os.Stat(latestSegment)
            if info != nil && time.Since(info.ModTime()) > 10*time.Second {
                w.logger.Warn("watchdog_stale_segments",
                    slog.String("stream_id", stream.ID),
                    slog.Duration("segment_age", time.Since(info.ModTime())),
                )
                // FFmpeg is running but not producing segments — likely stuck
                w.handleCrashedStream(ctx, stream)
            }
        }
    }
}

func (w *Watchdog) handleCrashedStream(ctx context.Context, stream *models.LiveStream) 
    w.logger.Warn("watchdog_ending_crashed_stream",
        slog.String("stream_id", stream.ID),
    )
    
    metrics.LiveStreamsTotal.WithLabelValues("error").Inc()
    w.liveService.EndStream(ctx, stream.ID, "watchdog_crash_detected")
}

// Check for VOD transcode jobs that have been "running" too long
func (w *Watchdog) checkStuckJobs(ctx context.Context) {
    stuckJobs, _ := w.jobRepo.FindStuck(ctx, 30*time.Minute) // running for >30 min
    
    for _, job := range stuckJobs {
        w.logger.Warn("watchdog_stuck_job",
            slog.String("job_id", job.ID),
            slog.String("video_id", job.VideoID),
            slog.Duration("running_for", time.Since(*job.StartedAt)),
        )
        
        // Reset to queued so it gets picked up again
        w.jobRepo.ResetToQueued(ctx, job.ID)
        metrics.TranscodeJobsTotal.WithLabelValues("reset_stuck").Inc()
    }
}

// Check for FFmpeg processes not tracked by any stream or job
func (w *Watchdog) checkOrphanedProcesses(ctx context.Context) {
    // List all FFmpeg PIDs on the system
    cmd := exec.CommandContext(ctx, "pgrep", "-f", "ffmpeg")
    output, err := cmd.Output()
    if err != nil {
        return // no ffmpeg processes, or pgrep not found
    }
    
    systemPIDs := make(map[int]bool)
    for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
        if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
            systemPIDs[pid] = true
        }
    }
    
    // Get all tracked PIDs (live streams + active jobs)
    trackedPIDs := w.getTrackedPIDs(ctx)
    
    // Find orphans
    for pid := range systemPIDs {
        if !trackedPIDs[pid] {
            w.logger.Warn("watchdog_orphaned_ffmpeg",
                slog.Int("pid", pid),
            )
            // Kill it — it's consuming resources for nothing
            if p, err := os.FindProcess(pid); err == nil {
                p.Signal(syscall.SIGTERM)
            }
        }
    }
    
    metrics.FFmpegProcesses.Set(float64(len(systemPIDs)))
}

func (w *Watchdog) findLatestSegment(hlsPath string) string {
    var latest string
    var latestTime time.Time
    
    filepath.Walk(hlsPath, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() { return nil }
        if strings.HasSuffix(path, ".m4s") && info.ModTime().After(latestTime) {
            latest = path
            latestTime = info.ModTime()
        }
        return nil
    })
    
    return latest
}
```

Add the repository method:
```go
// FindStuck returns jobs that have been in "running" status longer than maxDuration
func (r *JobRepo) FindStuck(ctx context.Context, maxDuration time.Duration) ([]*models.TranscodeJob, error) {
    cutoff := time.Now().Add(-maxDuration)
    rows, err := r.db.QueryContext(ctx,
        `SELECT id, video_id, status, started_at FROM transcode_jobs 
         WHERE status = 'running' AND started_at < $1`, cutoff)
    // ... scan rows ...
}

func (r *JobRepo) ResetToQueued(ctx context.Context, jobID string) error {
    _, err := r.db.ExecContext(ctx,
        `UPDATE transcode_jobs SET status = 'queued', started_at = NULL WHERE id = $1`, jobID)
    return err
}
```

---

### 6. Alerting Engine

A simple in-process alerting system that evaluates rules against current metrics and displays alerts on the dashboard.

```go
// internal/alerting/engine.go

type AlertSeverity string
const (
    SeverityCritical AlertSeverity = "critical"
    SeverityWarning  AlertSeverity = "warning"
    SeverityInfo     AlertSeverity = "info"
)

type AlertRule struct {
    Name        string
    Description string
    Severity    AlertSeverity
    Evaluate    func(m *qoe.DashboardMetrics, sys *SystemMetrics) bool
    Cooldown    time.Duration  // don't re-fire within this window
}

type Alert struct {
    ID          string        `json:"id"`
    RuleName    string        `json:"rule_name"`
    Description string        `json:"description"`
    Severity    AlertSeverity `json:"severity"`
    FiredAt     time.Time     `json:"fired_at"`
    ResolvedAt  *time.Time    `json:"resolved_at,omitempty"`
    IsActive    bool          `json:"is_active"`
}

type SystemMetrics struct {
    DiskAvailableGB     float64
    ActiveFFmpegProcs   int
    DBLatencyMs         int
    TranscodeQueueDepth int
}

type Engine struct {
    rules       []AlertRule
    activeAlerts map[string]*Alert  // rule_name → alert
    history     []*Alert            // last 100 alerts
    mu          sync.RWMutex
    qoe         *qoe.Aggregator
    logger      *slog.Logger
}

func NewEngine(qoeAgg *qoe.Aggregator) *Engine {
    e := &Engine{
        activeAlerts: make(map[string]*Alert),
        qoe:         qoeAgg,
        logger:      slog.With(slog.String("component", "alerting")),
    }
    e.registerDefaultRules()
    return e
}

func (e *Engine) registerDefaultRules() {
    e.rules = []AlertRule{
        // ── CRITICAL ──
        {
            Name:        "high_rebuffer_rate",
            Description: "Rebuffer rate exceeds 5% — viewers experiencing widespread buffering",
            Severity:    SeverityCritical,
            Cooldown:    2 * time.Minute,
            Evaluate: func(m *qoe.DashboardMetrics, sys *SystemMetrics) bool {
                return m.ActiveSessions > 0 && m.RebufferRate > 0.05
            },
        },
        {
            Name:        "high_join_failure",
            Description: "Playback start failures detected — viewers cannot watch videos",
            Severity:    SeverityCritical,
            Cooldown:    2 * time.Minute,
            Evaluate: func(m *qoe.DashboardMetrics, sys *SystemMetrics) bool {
                // Check from Prometheus counter, or derive from QoE data
                return false // implement based on your error tracking
            },
        },
        {
            Name:        "disk_space_critical",
            Description: "Less than 1 GB disk space remaining — uploads and recording will fail",
            Severity:    SeverityCritical,
            Cooldown:    5 * time.Minute,
            Evaluate: func(m *qoe.DashboardMetrics, sys *SystemMetrics) bool {
                return sys.DiskAvailableGB < 1.0
            },
        },
        {
            Name:        "database_unhealthy",
            Description: "PostgreSQL is slow or unreachable",
            Severity:    SeverityCritical,
            Cooldown:    1 * time.Minute,
            Evaluate: func(m *qoe.DashboardMetrics, sys *SystemMetrics) bool {
                return sys.DBLatencyMs > 1000 // >1 second
            },
        },
        
        // ── WARNING ──
        {
            Name:        "slow_ttff",
            Description: "Median time to first frame exceeds 2 seconds — startup experience degraded",
            Severity:    SeverityWarning,
            Cooldown:    5 * time.Minute,
            Evaluate: func(m *qoe.DashboardMetrics, sys *SystemMetrics) bool {
                return m.ActiveSessions > 0 && m.TTFFMedianMs > 2000
            },
        },
        {
            Name:        "transcode_queue_backed_up",
            Description: "Transcode queue depth exceeds 10 — processing delays expected",
            Severity:    SeverityWarning,
            Cooldown:    5 * time.Minute,
            Evaluate: func(m *qoe.DashboardMetrics, sys *SystemMetrics) bool {
                return sys.TranscodeQueueDepth > 10
            },
        },
        {
            Name:        "disk_space_warning",
            Description: "Less than 5 GB disk space remaining",
            Severity:    SeverityWarning,
            Cooldown:    10 * time.Minute,
            Evaluate: func(m *qoe.DashboardMetrics, sys *SystemMetrics) bool {
                return sys.DiskAvailableGB < 5.0 && sys.DiskAvailableGB >= 1.0
            },
        },
        {
            Name:        "high_quality_switches",
            Description: "Viewers experiencing frequent quality switches — network conditions may be poor",
            Severity:    SeverityWarning,
            Cooldown:    5 * time.Minute,
            Evaluate: func(m *qoe.DashboardMetrics, sys *SystemMetrics) bool {
                return m.ActiveSessions > 0 && m.QualitySwitchesPerMin > 3.0
            },
        },
        
        // ── INFO ──
        {
            Name:        "no_active_sessions",
            Description: "No viewers currently watching any content",
            Severity:    SeverityInfo,
            Cooldown:    30 * time.Minute,
            Evaluate: func(m *qoe.DashboardMetrics, sys *SystemMetrics) bool {
                return m.ActiveSessions == 0 && m.ActiveLiveStreams > 0
                // Only alert if there are live streams but nobody watching
            },
        },
    }
}

func (e *Engine) Start(ctx context.Context, sysMetricsFunc func() *SystemMetrics) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            e.evaluate(sysMetricsFunc())
        }
    }
}

func (e *Engine) evaluate(sys *SystemMetrics) {
    e.mu.Lock()
    defer e.mu.Unlock()
    
    qoeMetrics := e.qoe.GetMetrics()
    
    for _, rule := range e.rules {
        firing := rule.Evaluate(qoeMetrics, sys)
        existing, wasActive := e.activeAlerts[rule.Name]
        
        if firing && !wasActive {
            // New alert
            alert := &Alert{
                ID:          generateShortID(),
                RuleName:    rule.Name,
                Description: rule.Description,
                Severity:    rule.Severity,
                FiredAt:     time.Now(),
                IsActive:    true,
            }
            e.activeAlerts[rule.Name] = alert
            e.history = append(e.history, alert)
            if len(e.history) > 100 {
                e.history = e.history[1:]
            }
            
            e.logger.Warn("alert_fired",
                slog.String("rule", rule.Name),
                slog.String("severity", string(rule.Severity)),
                slog.String("description", rule.Description),
            )
            
        } else if !firing && wasActive {
            // Alert resolved
            now := time.Now()
            existing.ResolvedAt = &now
            existing.IsActive = false
            delete(e.activeAlerts, rule.Name)
            
            e.logger.Info("alert_resolved",
                slog.String("rule", rule.Name),
                slog.Duration("duration", now.Sub(existing.FiredAt)),
            )
        }
    }
}

func (e *Engine) GetActiveAlerts() []*Alert {
    e.mu.RLock()
    defer e.mu.RUnlock()
    
    alerts := make([]*Alert, 0, len(e.activeAlerts))
    for _, a := range e.activeAlerts {
        alerts = append(alerts, a)
    }
    // Sort by severity (critical first) then by time
    sort.Slice(alerts, func(i, j int) bool {
        if alerts[i].Severity != alerts[j].Severity {
            return severityRank(alerts[i].Severity) > severityRank(alerts[j].Severity)
        }
        return alerts[i].FiredAt.After(alerts[j].FiredAt)
    })
    return alerts
}

func (e *Engine) GetAlertHistory() []*Alert {
    e.mu.RLock()
    defer e.mu.RUnlock()
    result := make([]*Alert, len(e.history))
    copy(result, e.history)
    return result
}
```

#### Alert API

**GET /api/v1/alerts/active** — Current active alerts
```json
{
  "alerts": [
    {
      "id": "alt_abc",
      "rule_name": "high_rebuffer_rate",
      "description": "Rebuffer rate exceeds 5%...",
      "severity": "critical",
      "fired_at": "2025-01-15T20:30:00Z",
      "is_active": true
    }
  ]
}
```

**GET /api/v1/alerts/history** — Recent alert history (last 100)

#### Dashboard Update

Add an alert banner at the top of the QoE dashboard:

```html
<!-- Alert banner (appears when alerts are active) -->
<div id="alert-banner" style="display:none">
  <!-- Populated by JavaScript -->
</div>
```

```javascript
// In the SSE update handler, also fetch alerts
async function updateAlerts() {
  const res = await fetch('/api/v1/alerts/active');
  const data = await res.json();
  const banner = document.getElementById('alert-banner');
  
  if (data.alerts.length === 0) {
    banner.style.display = 'none';
    return;
  }
  
  banner.style.display = 'block';
  banner.innerHTML = data.alerts.map(a => `
    <div class="alert alert-${a.severity}">
      <span class="alert-severity">${a.severity.toUpperCase()}</span>
      <span class="alert-message">${a.description}</span>
      <span class="alert-time">${timeAgo(a.fired_at)}</span>
    </div>
  `).join('');
}

// Poll alerts every 10 seconds
setInterval(updateAlerts, 10000);
updateAlerts(); // initial load
```

CSS for alerts:
```css
.alert { padding: 10px 16px; margin-bottom: 4px; border-radius: 6px; display: flex; align-items: center; gap: 12px; }
.alert-critical { background: #fee2e2; border-left: 4px solid #dc2626; color: #991b1b; }
.alert-warning { background: #fef9c3; border-left: 4px solid #ca8a04; color: #854d0e; }
.alert-info { background: #dbeafe; border-left: 4px solid #2563eb; color: #1e40af; }
.alert-severity { font-weight: 700; font-size: 12px; text-transform: uppercase; min-width: 70px; }
.alert-time { margin-left: auto; font-size: 13px; opacity: 0.7; }
```

---

## Updated Project Structure

```
video-streaming/
├── cmd/server/main.go                   # Updated: graceful shutdown
├── internal/
│   ├── config/config.go                 # Add LOG_LEVEL, LOG_FORMAT
│   ├── database/postgres.go
│   ├── logging/                          # NEW
│   │   ├── setup.go                     # slog configuration
│   │   └── context.go                   # Context-aware logger
│   ├── metrics/                          # NEW
│   │   ├── metrics.go                   # All metric definitions
│   │   └── system_collector.go          # Background system metrics
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── metrics.go                   # NEW: HTTP metrics middleware
│   │   └── request_id.go               # NEW: Request ID injection
│   ├── health/                           # NEW
│   │   └── checker.go                   # Health check implementations
│   ├── watchdog/                         # NEW
│   │   └── watchdog.go                  # Process monitoring
│   ├── alerting/                         # NEW
│   │   └── engine.go                    # Alert rules + evaluation
│   ├── models/models.go
│   ├── repository/
│   │   ├── ... (existing)
│   │   └── instrumented.go              # NEW: DB query duration tracking wrapper
│   ├── service/
│   │   └── ... (existing, updated with metrics + context logging)
│   ├── handler/
│   │   ├── ... (existing, updated with metrics + context logging)
│   │   ├── health_handler.go            # NEW
│   │   └── alert_handler.go             # NEW
│   ├── live/
│   │   └── ... (existing, updated with metrics)
│   ├── qoe/aggregator.go               # Updated: expose for alerting
│   ├── worker/transcode_worker.go       # Updated: metrics + graceful drain
│   ├── transcoder/
│   │   └── ... (existing)
│   └── web/
│       ├── templates/
│       │   ├── dashboard.html           # Updated: alert banner
│       │   └── ... (existing)
│       └── embed.go
├── migrations/
│   ├── 001_initial.sql
│   ├── 002_sessions_and_events.sql
│   ├── 003_live_streaming.sql
│   └── (no new migration needed for Phase 5)
├── data/
├── docker-compose.yml                   # Add Prometheus + Grafana (optional)
├── prometheus.yml                       # NEW: Prometheus scrape config
├── go.mod, go.sum
├── Makefile
└── README.md
```

## New Dependencies

```bash
go get github.com/prometheus/client_golang
```

## Environment Variables (Updated)

```
PORT=8080
RTMP_PORT=1935
DATABASE_URL=postgres://videostream:videostream@localhost:5432/videostream?sslmode=disable
DATA_DIR=./data
WORKER_COUNT=2
JWT_SECRET=your-secret-key-change-in-production
JWT_EXPIRY=1h
LOG_LEVEL=info       # debug, info, warn, error
LOG_FORMAT=text      # text (dev), json (production)
```

## Optional: Prometheus + Grafana docker-compose addition

```yaml
# Add to docker-compose.yml for local metrics visualization
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
```

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'video-platform'
    static_configs:
      - targets: ['host.docker.internal:8080']  # or 'localhost:8080' on Linux
```

## Middleware Chain Order

The middleware must be applied in the correct order:

```go
// In route registration:
// 1. Request ID (outermost — adds ID for all downstream)
// 2. Metrics (records duration including all inner middleware)
// 3. Auth (only on protected routes)
// 4. Handler

// Public routes
mux.Handle("GET /health", requestID(metricsMiddleware(healthHandler)))
mux.Handle("GET /metrics", promhttp.Handler())  // no extra middleware

// Protected routes
mux.Handle("GET /videos/{video_id}/{rest...}", 
    requestID(metricsMiddleware(authMiddleware(videoFileHandler))))
```

## What This Phase Teaches Me

- How to instrument a Go application with Prometheus metrics (counters, gauges, histograms)
- How to avoid high-cardinality label explosions in Prometheus
- How structured logging with context correlation works in practice
- How health checks work for liveness vs readiness probes
- How graceful shutdown sequences prevent data loss and viewer disruption
- How process watchdogs detect and recover from FFmpeg crashes
- How alerting rules evaluate metrics and surface problems in real-time
- The full observability loop: instrument → collect → alert → display → respond

## Important Notes

- Instrument existing code by ADDING metric calls — don't restructure or rewrite working logic
- The metrics middleware MUST use categorized path patterns, never raw URLs (high cardinality kills Prometheus)
- Health checks should be fast (<2s total) — they may be called frequently by load balancers
- The graceful shutdown sequence ORDER matters: stop accepting → drain connections → wait for jobs → close DB
- The watchdog should be conservative: don't kill FFmpeg processes that might just be slow
- The alerting engine runs in-process (no external dependency) — it resets on restart, which is fine for this project
- Prometheus + Grafana in docker-compose is optional but nice for visualization
- Test graceful shutdown by running the server, starting a transcode, then pressing Ctrl+C