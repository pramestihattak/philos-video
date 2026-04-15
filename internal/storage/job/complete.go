package job

import (
	"context"
	"time"
)

func (r *Repo) Complete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE transcode_jobs SET status='completed', progress=1.0, updated_at=$1 WHERE id=$2`,
		time.Now(), id,
	)
	return err
}
