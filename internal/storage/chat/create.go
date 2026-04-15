package chat

import (
	"context"

	"philos-video/internal/models"
)

func (r *Repo) Create(ctx context.Context, m *models.ChatMessage) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO live_chat_messages (id, stream_id, user_id, body) VALUES ($1, $2, $3, $4)`,
		m.ID, m.StreamID, m.UserID, m.Body,
	)
	return err
}
