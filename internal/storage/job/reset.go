package job

import (
	"context"
	"time"
)

// ResetToQueued resets a job from 'running' back to 'queued' for retry.
func (r *Repo) ResetToQueued(ctx context.Context, jobID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE transcode_jobs SET status='queued', stage=NULL, updated_at=$1 WHERE id=$2`,
		time.Now(), jobID,
	)
	return err
}
