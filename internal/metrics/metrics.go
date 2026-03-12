package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTP metrics
var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path_pattern", "status_code"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "video_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path_pattern"})

	HTTPResponseBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_http_response_bytes_total",
		Help: "Total bytes sent in HTTP responses",
	}, []string{"method", "path_pattern"})
)

// Upload metrics
var (
	UploadsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_uploads_total",
		Help: "Total number of uploads by status",
	}, []string{"status"})

	UploadBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "video_upload_bytes_total",
		Help: "Total bytes received in chunk uploads",
	})

	UploadChunkDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "video_upload_chunk_duration_seconds",
		Help:    "Duration of individual chunk upload processing",
		Buckets: prometheus.DefBuckets,
	})

	ActiveUploads = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "video_active_uploads",
		Help: "Number of uploads currently in progress",
	})
)

// Transcode metrics
var (
	TranscodeQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "video_transcode_queue_depth",
		Help: "Number of jobs waiting in the transcode queue",
	})

	TranscodeJobsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_transcode_jobs_total",
		Help: "Total transcode jobs by status",
	}, []string{"status"})

	TranscodeJobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "video_transcode_job_duration_seconds",
		Help:    "Total duration of transcode jobs",
		Buckets: []float64{10, 30, 60, 120, 300, 600, 1200, 3600},
	}, []string{"source_resolution"})

	TranscodeEncodeDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "video_transcode_encode_duration_seconds",
		Help:    "Duration of individual encode passes",
		Buckets: []float64{5, 10, 30, 60, 120, 300, 600},
	}, []string{"resolution", "codec"})

	TranscodeActiveWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "video_transcode_active_workers",
		Help: "Number of workers currently processing jobs",
	})

	FFmpegProcesses = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "video_ffmpeg_processes",
		Help: "Number of currently running FFmpeg processes",
	})
)

// Live stream metrics
var (
	LiveStreamsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "video_live_streams_active",
		Help: "Number of currently active live streams",
	})

	LiveStreamsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_live_streams_total",
		Help: "Total live streams ended, by reason",
	}, []string{"end_reason"})

	LiveStreamDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "video_live_stream_duration_seconds",
		Help:    "Duration of live streams",
		Buckets: []float64{30, 60, 300, 600, 1800, 3600, 7200},
	})

	LiveIngestBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "video_live_ingest_bytes_total",
		Help: "Total bytes received from RTMP ingest",
	})

	LiveViewersActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "video_live_viewers_active",
		Help: "Number of active viewers per stream",
	}, []string{"stream_id"})

	RTMPConnectionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_rtmp_connections_total",
		Help: "Total RTMP connection attempts by result",
	}, []string{"result"})
)

// Delivery metrics
var (
	SegmentRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_segment_requests_total",
		Help: "Total HLS segment requests by type and quality",
	}, []string{"type", "quality"})

	SegmentRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "video_segment_request_duration_seconds",
		Help:    "HLS segment request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"type"})

	SegmentBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "video_segment_bytes_total",
		Help: "Total bytes served for HLS segments",
	})

	ManifestRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_manifest_requests_total",
		Help: "Total HLS manifest requests by type",
	}, []string{"type"})

	TokenValidationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_token_validations_total",
		Help: "Total JWT token validation attempts by result",
	}, []string{"result"})
)

// Playback QoE metrics
var (
	PlaybackSessionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "video_playback_sessions_active",
		Help: "Number of active playback sessions",
	})

	PlaybackSessionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_playback_sessions_total",
		Help: "Total playback sessions by type",
	}, []string{"type"})

	PlaybackTTFFSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "video_playback_ttff_seconds",
		Help:    "Time to first frame in seconds",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 3, 5, 10},
	})

	PlaybackRebufferTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "video_playback_rebuffer_events_total",
		Help: "Total rebuffering events",
	})

	PlaybackRebufferDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "video_playback_rebuffer_duration_seconds",
		Help:    "Duration of rebuffering events",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
	})

	PlaybackQualitySwitchesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "video_playback_quality_switches_total",
		Help: "Total quality switch events by direction",
	}, []string{"direction"})

	PlaybackBitrateKbps = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "video_playback_bitrate_kbps",
		Help:    "Observed playback bitrate in kbps",
		Buckets: []float64{100, 300, 500, 1000, 1500, 2500, 5000},
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

// System metrics
var (
	StorageBytesUsed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "video_storage_bytes_used",
		Help: "Disk space used per data directory",
	}, []string{"directory"})

	StorageBytesAvailable = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "video_storage_bytes_available",
		Help: "Available disk space on the data volume",
	})

	DatabaseConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "video_database_connections_active",
		Help: "Number of active database connections",
	})

	DatabaseQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "video_database_query_duration_seconds",
		Help:    "Database query duration by operation",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})

	ServerUptimeSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "video_server_uptime_seconds",
		Help: "Server uptime in seconds",
	})
)
