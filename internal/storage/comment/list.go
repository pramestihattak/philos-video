package comment

import (
	"context"

	"philos-video/internal/models"
)

func (r *Repo) ListByVideo(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.video_id, c.user_id, COALESCE(u.name,''), COALESCE(u.picture,''), c.body, c.created_at
		FROM comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.video_id = $1
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3`,
		videoID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.Comment
	for rows.Next() {
		var c models.Comment
		if err := rows.Scan(&c.ID, &c.VideoID, &c.UserID, &c.UserName, &c.UserPic, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}
