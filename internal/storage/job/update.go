package job

import (
	"context"
	"time"
)

func (r *Repo) UpdateRunning(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE transcode_jobs SET status='running', updated_at=$1 WHERE id=$2`,
		time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateProgress(ctx context.Context, id, stage string, progress float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE transcode_jobs SET stage=$1, progress=$2, updated_at=$3 WHERE id=$4`,
		stage, progress, time.Now(), id,
	)
	return err
}
