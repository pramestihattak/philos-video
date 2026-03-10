package repository

import (
	"context"
	"database/sql"
	"time"

	"philos-video/internal/models"
)

type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(ctx context.Context, s *models.PlaybackSession) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO playback_sessions (id, video_id, stream_id, token, device_type, user_agent, ip_address)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.ID, ns(s.VideoID), ns(s.StreamID), s.Token, ns(s.DeviceType), ns(s.UserAgent), ns(s.IPAddress),
	)
	return err
}

func (r *SessionRepo) Get(ctx context.Context, id string) (*models.PlaybackSession, error) {
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

func (r *SessionRepo) TouchLastActive(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE playback_sessions SET last_active_at=$1 WHERE id=$2`,
		time.Now(), id,
	)
	return err
}

func (r *SessionRepo) MarkEnded(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE playback_sessions SET ended_at=$1, status='ended' WHERE id=$2`,
		now, id,
	)
	return err
}
