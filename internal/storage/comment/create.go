package comment

import (
	"context"

	"philos-video/internal/models"
)

func (r *Repo) Create(ctx context.Context, c *models.Comment) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO comments (id, video_id, user_id, body) VALUES ($1, $2, $3, $4)`,
		c.ID, c.VideoID, c.UserID, c.Body,
	)
	return err
}
