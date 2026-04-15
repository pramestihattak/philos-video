package video

import (
	"context"
	"database/sql"
)

// Delete removes a video and all dependent rows in a transaction.
// It requires the owning userID to prevent cross-user deletes.
func (r *Repo) Delete(ctx context.Context, id, userID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Confirm ownership before touching anything.
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT TRUE FROM videos WHERE id=$1 AND user_id=$2`, id, userID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return nil // not found or not owner — treat as no-op
		}
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM comments WHERE video_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM playback_events pe
		USING playback_sessions ps
		WHERE pe.session_id = ps.id AND ps.video_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM playback_sessions WHERE video_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM transcode_jobs WHERE video_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM videos WHERE id = $1`, id); err != nil {
		return err
	}

	return tx.Commit()
}
