package job

import (
	"context"
	"time"
)

func (r *Repo) Fail(ctx context.Context, id, errMsg string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE transcode_jobs SET status='failed', error=$1, updated_at=$2 WHERE id=$3`,
		errMsg, time.Now(), id,
	)
	return err
}
