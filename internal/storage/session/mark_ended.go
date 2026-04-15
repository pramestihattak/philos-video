package session

import (
	"context"
	"time"
)

func (r *Repo) MarkEnded(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE playback_sessions SET ended_at=$1, status='ended' WHERE id=$2`,
		time.Now(), id,
	)
	return err
}
