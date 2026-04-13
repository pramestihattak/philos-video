package models

import "time"

const (
	VideoStatusUploading  = "uploading"
	VideoStatusProcessing = "processing"
	VideoStatusReady      = "ready"
	VideoStatusFailed     = "failed"

	VisibilityPrivate  = "private"
	VisibilityUnlisted = "unlisted"
	VisibilityPublic   = "public"

	JobStatusQueued    = "queued"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"

	StreamStatusWaiting = "waiting"
	StreamStatusLive    = "live"
	StreamStatusEnded   = "ended"
)

type Video struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id,omitempty"`
	UploaderName    string    `json:"uploader_name,omitempty"`
	UploaderPicture string    `json:"uploader_picture,omitempty"`
	Title         string    `json:"title"`
	Visibility    string    `json:"visibility"`
	Status        string    `json:"status"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	Duration      string    `json:"duration"`
	Codec         string    `json:"codec"`
	HLSPath       string    `json:"hls_path"`
	SizeBytes     int64     `json:"size_bytes,omitempty"`
	ThumbnailPath string    `json:"thumbnail_path,omitempty"`
	PlayCount     int       `json:"play_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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
	VideoID      string     `json:"video_id,omitempty"`  // set for VOD sessions
	StreamID     string     `json:"stream_id,omitempty"` // set for live sessions
	Token        string     `json:"token,omitempty"`
	DeviceType   string     `json:"device_type,omitempty"`
	UserAgent    string     `json:"user_agent,omitempty"`
	IPAddress    string     `json:"ip_address,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	LastActiveAt time.Time  `json:"last_active_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	Status       string     `json:"status"`
}

type StreamKey struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id,omitempty"`
	UserLabel string    `json:"user_label"`
	IsActive  bool      `json:"is_active"`
	RecordVOD bool      `json:"record_vod"`
	CreatedAt time.Time `json:"created_at"`
}

type LiveStream struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id,omitempty"`
	StreamKeyID  string     `json:"stream_key_id"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	RecordVOD    bool       `json:"record_vod"`
	SourceWidth  int        `json:"source_width,omitempty"`
	SourceHeight int        `json:"source_height,omitempty"`
	SourceCodec  string     `json:"source_codec,omitempty"`
	SourceFPS    string     `json:"source_fps,omitempty"`
	HLSPath      string     `json:"hls_path,omitempty"`
	VideoID      string     `json:"video_id,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
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
