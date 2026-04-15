package video

import (
	"context"
	"database/sql"

	"philos-video/internal/models"
)

// GetByID returns a video by ID regardless of owner (used by public playback paths).
func (r *Repo) GetByID(ctx context.Context, id string) (*models.Video, error) {
	row := r.db.QueryRowContext(ctx, getByIDQuery, id)
	v, err := scanVideo(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return v, err
}

// GetByIDForUser returns a video only if it belongs to the given user.
func (r *Repo) GetByIDForUser(ctx context.Context, id, userID string) (*models.Video, error) {
	row := r.db.QueryRowContext(ctx, getByIDQuery+` AND v.user_id = $2`, id, userID)
	v, err := scanVideo(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return v, err
}
