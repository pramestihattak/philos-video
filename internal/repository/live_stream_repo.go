package repository

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	"philos-video/internal/models"
)

type LiveStreamRepo struct {
	db *sql.DB
}

func NewLiveStreamRepo(db *sql.DB) *LiveStreamRepo {
	return &LiveStreamRepo{db: db}
}

func scanLiveStream(row interface {
	Scan(...any) error
}) (*models.LiveStream, error) {
	ls := &models.LiveStream{}
	var sourceWidth, sourceHeight sql.NullInt64
	var sourceCodec, sourceFPS, hlsPath, videoID sql.NullString
	err := row.Scan(
		&ls.ID, &ls.StreamKeyID, &ls.Title, &ls.Status,
		&sourceWidth, &sourceHeight, &sourceCodec, &sourceFPS,
		&hlsPath, &videoID, &ls.StartedAt, &ls.EndedAt, &ls.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if sourceWidth.Valid {
		ls.SourceWidth = int(sourceWidth.Int64)
	}
	if sourceHeight.Valid {
		ls.SourceHeight = int(sourceHeight.Int64)
	}
	if sourceCodec.Valid {
		ls.SourceCodec = sourceCodec.String
	}
	if sourceFPS.Valid {
		ls.SourceFPS = sourceFPS.String
	}
	if hlsPath.Valid {
		ls.HLSPath = hlsPath.String
	}
	if videoID.Valid {
		ls.VideoID = videoID.String
	}
	return ls, nil
}

const liveStreamCols = `id, stream_key_id, title, status,
    source_width, source_height, source_codec, source_fps,
    hls_path, video_id, started_at, ended_at, created_at`

func (r *LiveStreamRepo) Create(streamKeyID, title string) (*models.LiveStream, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating stream id: %w", err)
	}
	id := "ls_" + hex.EncodeToString(b)

	row := r.db.QueryRow(
		`INSERT INTO live_streams (id, stream_key_id, title) VALUES ($1, $2, $3)
		 RETURNING `+liveStreamCols,
		id, streamKeyID, title,
	)
	ls, err := scanLiveStream(row)
	if err != nil {
		return nil, fmt.Errorf("creating live stream: %w", err)
	}
	return ls, nil
}

func (r *LiveStreamRepo) GetByID(id string) (*models.LiveStream, error) {
	row := r.db.QueryRow(
		`SELECT `+liveStreamCols+` FROM live_streams WHERE id = $1`, id,
	)
	ls, err := scanLiveStream(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting live stream: %w", err)
	}
	return ls, nil
}

func (r *LiveStreamRepo) GetActiveByStreamKey(streamKeyID string) (*models.LiveStream, error) {
	row := r.db.QueryRow(
		`SELECT `+liveStreamCols+` FROM live_streams
		 WHERE stream_key_id = $1 AND status IN ('waiting', 'live')
		 ORDER BY created_at DESC LIMIT 1`,
		streamKeyID,
	)
	ls, err := scanLiveStream(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting active live stream: %w", err)
	}
	return ls, nil
}

func (r *LiveStreamRepo) ListLive() ([]*models.LiveStream, error) {
	rows, err := r.db.Query(
		`SELECT ` + liveStreamCols + ` FROM live_streams WHERE status = 'live' ORDER BY started_at DESC`,
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

func (r *LiveStreamRepo) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(`UPDATE live_streams SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *LiveStreamRepo) UpdateStarted(id string) error {
	_, err := r.db.Exec(
		`UPDATE live_streams SET status = 'live', started_at = NOW() WHERE id = $1`, id,
	)
	return err
}

func (r *LiveStreamRepo) UpdateEnded(id string) error {
	_, err := r.db.Exec(
		`UPDATE live_streams SET status = 'ended', ended_at = NOW() WHERE id = $1`, id,
	)
	return err
}

func (r *LiveStreamRepo) UpdateHLSPath(id, hlsPath string) error {
	_, err := r.db.Exec(`UPDATE live_streams SET hls_path = $1 WHERE id = $2`, hlsPath, id)
	return err
}

func (r *LiveStreamRepo) UpdateVideoID(id, videoID string) error {
	_, err := r.db.Exec(`UPDATE live_streams SET video_id = $1 WHERE id = $2`, videoID, id)
	return err
}

func (r *LiveStreamRepo) UpdateSourceInfo(id string, width, height int, codec, fps string) error {
	_, err := r.db.Exec(
		`UPDATE live_streams SET source_width=$1, source_height=$2, source_codec=$3, source_fps=$4 WHERE id=$5`,
		width, height, codec, fps, id,
	)
	return err
}
