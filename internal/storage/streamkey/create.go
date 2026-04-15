package streamkey

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"philos-video/internal/models"
)

func (r *Repo) Create(ctx context.Context, label string, recordVOD bool, userID string) (*models.StreamKey, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating stream key id: %w", err)
	}
	id := "sk_" + hex.EncodeToString(b)

	sk := &models.StreamKey{}
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO stream_keys (id, user_id, user_label, record_vod) VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, user_label, is_active, record_vod, created_at`,
		id, userID, label, recordVOD,
	).Scan(&sk.ID, &sk.UserID, &sk.UserLabel, &sk.IsActive, &sk.RecordVOD, &sk.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating stream key: %w", err)
	}
	return sk, nil
}
