package session

import "context"

// CountActiveByStreamID returns the number of viewers active in the last 2 minutes.
func (r *Repo) CountActiveByStreamID(ctx context.Context, streamID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM playback_sessions
		 WHERE stream_id = $1
		   AND status != 'ended'
		   AND last_active_at > NOW() - INTERVAL '2 minutes'`,
		streamID,
	).Scan(&count)
	return count, err
}
