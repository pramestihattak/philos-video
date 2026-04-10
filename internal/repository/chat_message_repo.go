package repository

import (
	"context"
	"database/sql"

	"philos-video/internal/models"
)

type ChatMessageRepo struct {
	db *sql.DB
}

func NewChatMessageRepo(db *sql.DB) *ChatMessageRepo {
	return &ChatMessageRepo{db: db}
}

func (r *ChatMessageRepo) Create(ctx context.Context, m *models.ChatMessage) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO live_chat_messages (id, stream_id, user_id, body) VALUES ($1, $2, $3, $4)`,
		m.ID, m.StreamID, m.UserID, m.Body,
	)
	return err
}

func (r *ChatMessageRepo) ListByStream(ctx context.Context, streamID string, limit, offset int) ([]*models.ChatMessage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.id, m.stream_id, m.user_id, COALESCE(u.name,''), COALESCE(u.picture,''), m.body, m.created_at
		FROM live_chat_messages m
		JOIN users u ON u.id = m.user_id
		WHERE m.stream_id = $1
		ORDER BY m.created_at ASC
		LIMIT $2 OFFSET $3`,
		streamID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.ChatMessage
	for rows.Next() {
		var m models.ChatMessage
		if err := rows.Scan(&m.ID, &m.StreamID, &m.UserID, &m.UserName, &m.UserPic, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}
