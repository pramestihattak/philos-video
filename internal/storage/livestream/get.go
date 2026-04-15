package livestream

import (
	"context"
	"database/sql"
	"fmt"

	"philos-video/internal/models"
)

// GetByID is unscoped — used by public viewer paths and RTMP lifecycle callbacks.
func (r *Repo) GetByID(ctx context.Context, id string) (*models.LiveStream, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+cols+` FROM live_streams WHERE id = $1`, id)
	ls, err := scanLiveStream(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting live stream: %w", err)
	}
	return ls, nil
}

// GetByIDForUser returns a stream only if it belongs to the given user.
func (r *Repo) GetByIDForUser(ctx context.Context, id, userID string) (*models.LiveStream, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+cols+` FROM live_streams WHERE id = $1 AND user_id = $2`, id, userID,
	)
	ls, err := scanLiveStream(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting live stream for user: %w", err)
	}
	return ls, nil
}

func (r *Repo) GetActiveByStreamKey(ctx context.Context, streamKeyID string) (*models.LiveStream, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+cols+` FROM live_streams
		 WHERE stream_key_id = $1 AND status IN ('waiting', 'live')
		 ORDER BY created_at DESC LIMIT 1`,
		streamKeyID,
	)
	ls, err := scanLiveStream(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting active live stream: %w", err)
	}
	return ls, nil
}
