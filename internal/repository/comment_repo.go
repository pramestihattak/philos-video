package repository

import (
	"context"
	"database/sql"

	"philos-video/internal/models"
)

type CommentRepo struct {
	db *sql.DB
}

func NewCommentRepo(db *sql.DB) *CommentRepo {
	return &CommentRepo{db: db}
}

func (r *CommentRepo) Create(ctx context.Context, c *models.Comment) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO comments (id, video_id, user_id, body) VALUES ($1, $2, $3, $4)`,
		c.ID, c.VideoID, c.UserID, c.Body,
	)
	return err
}

func (r *CommentRepo) ListByVideo(ctx context.Context, videoID string, limit, offset int) ([]*models.Comment, error) {
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

// Delete removes a comment only if it belongs to userID.
func (r *CommentRepo) Delete(ctx context.Context, id, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM comments WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

// DeleteByVideo removes all comments for a video (used in VideoRepo.Delete transaction).
func (r *CommentRepo) DeleteByVideo(ctx context.Context, videoID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM comments WHERE video_id = $1`,
		videoID,
	)
	return err
}
