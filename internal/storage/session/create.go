package session

import (
	"context"

	"philos-video/internal/models"
	"philos-video/internal/storage"
)

func (r *Repo) Create(ctx context.Context, s *models.PlaybackSession) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO playback_sessions (id, video_id, stream_id, token, device_type, user_agent, ip_address)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.ID, storage.Ns(s.VideoID), storage.Ns(s.StreamID), s.Token,
		storage.Ns(s.DeviceType), storage.Ns(s.UserAgent), storage.Ns(s.IPAddress),
	)
	return err
}
