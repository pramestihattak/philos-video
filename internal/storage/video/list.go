package video

import (
	"context"

	"philos-video/internal/models"
)

// ListPublic returns public ready videos visible to guests.
func (r *Repo) ListPublic(ctx context.Context, limit, offset int) ([]*models.Video, error) {
	rows, err := r.db.QueryContext(ctx, listPublicQuery, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

// List returns up to limit videos for the given user, ordered by creation time descending.
func (r *Repo) List(ctx context.Context, limit, offset int, userID string) ([]*models.Video, error) {
	rows, err := r.db.QueryContext(ctx, listQuery, limit, offset, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}
