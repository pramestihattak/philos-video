package comment

import "context"

// Delete removes a comment only if it belongs to userID.
func (r *Repo) Delete(ctx context.Context, id, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM comments WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

// DeleteByVideo removes all comments for a video (used in video delete transaction).
func (r *Repo) DeleteByVideo(ctx context.Context, videoID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM comments WHERE video_id = $1`,
		videoID,
	)
	return err
}
