package job

import (
	"context"
	"time"

	"philos-video/internal/models"
)

// Repository defines the persistence operations for transcode jobs.
type Repository interface {
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
