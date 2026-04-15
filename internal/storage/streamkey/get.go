package streamkey

import (
	"context"
	"database/sql"
	"fmt"

	"philos-video/internal/models"
)

// GetByID is intentionally unscoped — used by RTMP ingest which only has the key secret.
func (r *Repo) GetByID(ctx context.Context, id string) (*models.StreamKey, error) {
	sk := &models.StreamKey{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, user_label, is_active, record_vod, created_at FROM stream_keys WHERE id = $1`, id,
	).Scan(&sk.ID, &sk.UserID, &sk.UserLabel, &sk.IsActive, &sk.RecordVOD, &sk.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting stream key: %w", err)
	}
	return sk, nil
}
