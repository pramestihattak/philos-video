package streamkey

import (
	"context"
	"fmt"

	"philos-video/internal/models"
)

// List returns all active stream keys for a specific user.
func (r *Repo) List(ctx context.Context, userID string) ([]*models.StreamKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, user_label, is_active, record_vod, created_at
		 FROM stream_keys WHERE user_id = $1 AND is_active = TRUE ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing stream keys: %w", err)
	}
	defer rows.Close()

	var keys []*models.StreamKey
	for rows.Next() {
		sk := &models.StreamKey{}
		if err := rows.Scan(&sk.ID, &sk.UserID, &sk.UserLabel, &sk.IsActive, &sk.RecordVOD, &sk.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, sk)
	}
	return keys, rows.Err()
}
