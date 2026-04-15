package livestream

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"philos-video/internal/models"
)

func (r *Repo) Create(ctx context.Context, streamKeyID, title string, recordVOD bool, userID string) (*models.LiveStream, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating stream id: %w", err)
	}
	id := "ls_" + hex.EncodeToString(b)

	row := r.db.QueryRowContext(ctx,
		`INSERT INTO live_streams (id, user_id, stream_key_id, title, record_vod) VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+cols,
		id, userID, streamKeyID, title, recordVOD,
	)
	ls, err := scanLiveStream(row)
	if err != nil {
		return nil, fmt.Errorf("creating live stream: %w", err)
	}
	return ls, nil
}
