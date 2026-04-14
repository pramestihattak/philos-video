package storage

import (
	"context"
	"time"

	"philos-video/internal/models"
)

// VideoStorer defines the persistence operations for videos.
type VideoStorer interface {
	Create(ctx context.Context, v *models.Video) error
	GetByID(ctx context.Context, id string) (*models.Video, error)
	GetByIDForUser(ctx context.Context, id, userID string) (*models.Video, error)
	ListPublic(ctx context.Context, limit, offset int) ([]*models.Video, error)
	List(ctx context.Context, limit, offset int, userID string) ([]*models.Video, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateAfterProbe(ctx context.Context, id string, width, height int, duration, codec string) error
	UpdateHLSPath(ctx context.Context, id, hlsPath string) error
	UpdateSizeBytes(ctx context.Context, id string, size int64) error
	UpdateThumbnailPath(ctx context.Context, id, thumbnailPath string) error
	UpdateVisibility(ctx context.Context, id, userID, visibility string) error
	Delete(ctx context.Context, id, userID string) error
}

// UploadStorer defines the persistence operations for chunked uploads.
type UploadStorer interface {
	CreateChunks(ctx context.Context, uploadID string, totalChunks int) error
	MarkChunkReceived(ctx context.Context, uploadID string, chunkNumber int) error
	GetProgress(ctx context.Context, uploadID string) (received, total int, err error)
}

// JobStorer defines the persistence operations for transcode jobs.
type JobStorer interface {
	Create(ctx context.Context, job *models.TranscodeJob) error
	GetByID(ctx context.Context, id string) (*models.TranscodeJob, error)
	GetByVideoID(ctx context.Context, videoID string) (*models.TranscodeJob, error)
	UpdateRunning(ctx context.Context, id string) error
	UpdateProgress(ctx context.Context, id, stage string, progress float64) error
	Complete(ctx context.Context, id string) error
	Fail(ctx context.Context, id, errMsg string) error
	ListQueued(ctx context.Context) ([]string, error)
	FindStuck(ctx context.Context, d time.Duration) ([]*models.TranscodeJob, error)
	ResetToQueued(ctx context.Context, jobID string) error
}

// StreamKeyStorer defines the persistence operations for stream keys.
type StreamKeyStorer interface {
	Create(ctx context.Context, label string, recordVOD bool, userID string) (*models.StreamKey, error)
	GetByID(ctx context.Context, id string) (*models.StreamKey, error)
	List(ctx context.Context, userID string) ([]*models.StreamKey, error)
	Deactivate(ctx context.Context, id, userID string) error
	UpdateRecordVOD(ctx context.Context, id string, recordVOD bool, userID string) error
}

// LiveStreamStorer defines the persistence operations for live streams.
type LiveStreamStorer interface {
	Create(ctx context.Context, streamKeyID, title string, recordVOD bool, userID string) (*models.LiveStream, error)
	GetByID(ctx context.Context, id string) (*models.LiveStream, error)
	GetByIDForUser(ctx context.Context, id, userID string) (*models.LiveStream, error)
	GetActiveByStreamKey(ctx context.Context, streamKeyID string) (*models.LiveStream, error)
	ListLive(ctx context.Context) ([]*models.LiveStream, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateStarted(ctx context.Context, id string) error
	UpdateEnded(ctx context.Context, id string) error
	UpdateHLSPath(ctx context.Context, id, hlsPath string) error
	UpdateVideoID(ctx context.Context, id, videoID string) error
	UpdateSourceInfo(ctx context.Context, id string, width, height int, codec, fps string) error
}

// UserStorer defines the persistence operations for users.
type UserStorer interface {
	UpsertFromGoogle(ctx context.Context, googleSub, email, name, picture string, defaultQuota int64) (*models.User, bool, error)
	GetByID(ctx context.Context, id string) (*models.User, error)
	HasQuotaFor(ctx context.Context, userID string, size int64) (bool, error)
	IncUsedBytes(ctx context.Context, userID string, delta int64) error
	DecUsedBytes(ctx context.Context, userID string, delta int64) error
}

// SessionStorer defines the persistence operations for playback sessions.
type SessionStorer interface {
	Create(ctx context.Context, s *models.PlaybackSession) error
	Get(ctx context.Context, id string) (*models.PlaybackSession, error)
	TouchLastActive(ctx context.Context, id string) error
	MarkEnded(ctx context.Context, id string) error
	CountActiveByStreamID(ctx context.Context, streamID string) (int, error)
}

// EventStorer defines the persistence operations for playback events.
type EventStorer interface {
	BatchInsert(ctx context.Context, events []models.PlaybackEvent) error
}

// CommentStorer defines the persistence operations for comments.
type CommentStorer interface {
	Create(ctx context.Context, c *models.Comment) error
	ListByVideo(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error)
	Delete(ctx context.Context, id, userID string) error
	DeleteByVideo(ctx context.Context, videoID string) error
}

// ChatMessageStorer defines the persistence operations for live chat messages.
type ChatMessageStorer interface {
	Create(ctx context.Context, m *models.ChatMessage) error
	ListByStream(ctx context.Context, streamID string, limit, offset int) ([]*models.ChatMessage, error)
}
