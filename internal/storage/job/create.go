package job

import (
	"context"

	"philos-video/internal/models"
)

func (r *Repo) Create(ctx context.Context, j *models.TranscodeJob) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO transcode_jobs (id, video_id, status) VALUES ($1, $2, $3)`,
		j.ID, j.VideoID, j.Status,
	)
	return err
}
