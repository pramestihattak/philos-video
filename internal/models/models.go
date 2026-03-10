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
