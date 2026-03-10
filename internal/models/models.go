package models

import "time"

const (
	VideoStatusUploading  = "uploading"
	VideoStatusProcessing = "processing"
	VideoStatusReady      = "ready"
	VideoStatusFailed     = "failed"

	JobStatusQueued    = "queued"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
)

type Video struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Duration  string    `json:"duration"`
	Codec     string    `json:"codec"`
	HLSPath   string    `json:"hls_path"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UploadChunk struct {
	UploadID    string
	ChunkNumber int
	Received    bool
}

type TranscodeJob struct {
	ID        string    `json:"id"`
	VideoID   string    `json:"video_id"`
	Status    string    `json:"status"`
	Stage     string    `json:"stage"`
	Progress  float64   `json:"progress"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PlaybackSession struct {
	ID           string     `json:"id"`
	VideoID      string     `json:"video_id"`
	Token        string     `json:"token,omitempty"`
	DeviceType   string     `json:"device_type,omitempty"`
	UserAgent    string     `json:"user_agent,omitempty"`
	IPAddress    string     `json:"ip_address,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	LastActiveAt time.Time  `json:"last_active_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	Status       string     `json:"status"`
}

type PlaybackEvent struct {
	ID        int64     `json:"id,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	VideoID   string    `json:"video_id,omitempty"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`

	// Segment metrics
	SegmentNumber  *int    `json:"segment_number,omitempty"`
	SegmentQuality string  `json:"segment_quality,omitempty"`
	SegmentBytes   *int64  `json:"segment_bytes,omitempty"`
	DownloadTimeMs *int    `json:"download_time_ms,omitempty"`
	ThroughputBps  *int64  `json:"throughput_bps,omitempty"`

	// Playback state
	CurrentQuality   string   `json:"current_quality,omitempty"`
	BufferLength     *float64 `json:"buffer_length,omitempty"`
	PlaybackPosition *float64 `json:"playback_position,omitempty"`

	// Experience events
	RebufferDurationMs *int   `json:"rebuffer_duration_ms,omitempty"`
	QualityFrom        string `json:"quality_from,omitempty"`
	QualityTo          string `json:"quality_to,omitempty"`

	// Error
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}
