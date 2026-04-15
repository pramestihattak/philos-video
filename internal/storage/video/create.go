package video

import (
	"context"

	"philos-video/internal/models"
)

func (r *Repo) Create(ctx context.Context, v *models.Video) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO videos (id, user_id, title, visibility, status) VALUES ($1, $2, $3, $4, $5)`,
		v.ID, v.UserID, v.Title, v.Visibility, v.Status,
	)
	return err
}
