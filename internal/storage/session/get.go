package session

import (
	"context"
	"database/sql"

	"philos-video/internal/models"
)

func (r *Repo) Get(ctx context.Context, id string) (*models.PlaybackSession, error) {
	s := &models.PlaybackSession{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, COALESCE(video_id,''), COALESCE(stream_id,''), token,
		        COALESCE(device_type,''), COALESCE(user_agent,''), COALESCE(ip_address,''),
		        started_at, last_active_at, ended_at, status
		 FROM playback_sessions WHERE id=$1`, id,
	).Scan(&s.ID, &s.VideoID, &s.StreamID, &s.Token,
		&s.DeviceType, &s.UserAgent, &s.IPAddress,
		&s.StartedAt, &s.LastActiveAt, &s.EndedAt, &s.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}
