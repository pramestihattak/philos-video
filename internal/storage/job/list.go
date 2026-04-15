package job

import (
	"context"
	"time"

	"philos-video/internal/models"
)

// ListQueued returns job IDs that are in 'queued' status, ordered oldest first.
func (r *Repo) ListQueued(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM transcode_jobs WHERE status='queued' ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindStuck returns jobs that have been in 'running' status for longer than d.
func (r *Repo) FindStuck(ctx context.Context, d time.Duration) ([]*models.TranscodeJob, error) {
	cutoff := time.Now().Add(-d)
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+jobCols+` FROM transcode_jobs WHERE status='running' AND updated_at < $1`, cutoff,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.TranscodeJob
	for rows.Next() {
		j := &models.TranscodeJob{}
		if err := rows.Scan(&j.ID, &j.VideoID, &j.Status, &j.Stage, &j.Progress, &j.Error,
			&j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
