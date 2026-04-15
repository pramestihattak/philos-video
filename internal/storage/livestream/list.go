package livestream

import (
	"context"
	"fmt"

	"philos-video/internal/models"
)

func (r *Repo) ListLive(ctx context.Context) ([]*models.LiveStream, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+cols+` FROM live_streams WHERE status = 'live' ORDER BY started_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing live streams: %w", err)
	}
	defer rows.Close()

	var streams []*models.LiveStream
	for rows.Next() {
		ls, err := scanLiveStream(rows)
		if err != nil {
			return nil, err
		}
		streams = append(streams, ls)
	}
	return streams, rows.Err()
}
