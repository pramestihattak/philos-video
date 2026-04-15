package chat

import (
	"context"

	"philos-video/internal/models"
)

func (r *Repo) ListByStream(ctx context.Context, streamID string, limit, offset int) ([]*models.ChatMessage, error) {
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
