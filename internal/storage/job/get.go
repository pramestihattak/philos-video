package job

import (
	"context"
	"database/sql"

	"philos-video/internal/models"
)

func (r *Repo) GetByID(ctx context.Context, id string) (*models.TranscodeJob, error) {
	j := &models.TranscodeJob{}
	err := r.db.QueryRowContext(ctx,
		`SELECT `+jobCols+` FROM transcode_jobs WHERE id=$1`, id,
	).Scan(&j.ID, &j.VideoID, &j.Status, &j.Stage, &j.Progress, &j.Error, &j.CreatedAt, &j.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return j, err
}

func (r *Repo) GetByVideoID(ctx context.Context, videoID string) (*models.TranscodeJob, error) {
	j := &models.TranscodeJob{}
	err := r.db.QueryRowContext(ctx,
		`SELECT `+jobCols+` FROM transcode_jobs WHERE video_id=$1 ORDER BY created_at DESC LIMIT 1`, videoID,
	).Scan(&j.ID, &j.VideoID, &j.Status, &j.Stage, &j.Progress, &j.Error, &j.CreatedAt, &j.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return j, err
}
