package repository

import (
	"database/sql"
	"time"

	"philos-video/internal/models"
)

type VideoRepo struct {
	db *sql.DB
}

func NewVideoRepo(db *sql.DB) *VideoRepo {
	return &VideoRepo{db: db}
}

func (r *VideoRepo) Create(v *models.Video) error {
	_, err := r.db.Exec(
		`INSERT INTO videos (id, title, status) VALUES ($1, $2, $3)`,
		v.ID, v.Title, v.Status,
	)
	return err
}

// getByIDQuery fetches a single video by ID using a correlated subquery for play count
// (single-row lookup — subquery cost is acceptable here).
const getByIDQuery = `
	SELECT v.id, v.title, v.status,
	       COALESCE(v.width,0), COALESCE(v.height,0),
	       COALESCE(v.duration,''), COALESCE(v.codec,''), COALESCE(v.hls_path,''),
	       (SELECT COUNT(*) FROM playback_sessions ps WHERE ps.video_id = v.id),
	       v.created_at, v.updated_at
	FROM videos v
	WHERE v.id = $1`

// listQuery uses a LEFT JOIN + GROUP BY to fetch play counts in a single round-trip,
// avoiding the N+1 correlated subquery that fires once per video in the list.
const listQuery = `
	SELECT v.id, v.title, v.status,
	       COALESCE(v.width,0), COALESCE(v.height,0),
	       COALESCE(v.duration,''), COALESCE(v.codec,''), COALESCE(v.hls_path,''),
	       COUNT(ps.id),
	       v.created_at, v.updated_at
	FROM videos v
	LEFT JOIN playback_sessions ps ON ps.video_id = v.id
	GROUP BY v.id
	ORDER BY v.created_at DESC
	LIMIT $1 OFFSET $2`

func scanVideo(s interface{ Scan(...any) error }) (*models.Video, error) {
	v := &models.Video{}
	err := s.Scan(&v.ID, &v.Title, &v.Status, &v.Width, &v.Height,
		&v.Duration, &v.Codec, &v.HLSPath, &v.PlayCount, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func (r *VideoRepo) GetByID(id string) (*models.Video, error) {
	row := r.db.QueryRow(getByIDQuery, id)
	v, err := scanVideo(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return v, err
}

// List returns up to limit videos starting at offset, ordered by creation time descending.
func (r *VideoRepo) List(limit, offset int) ([]*models.Video, error) {
	rows, err := r.db.Query(listQuery, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

func (r *VideoRepo) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(
		`UPDATE videos SET status=$1, updated_at=$2 WHERE id=$3`,
		status, time.Now(), id,
	)
	return err
}

func (r *VideoRepo) UpdateAfterProbe(id string, width, height int, duration, codec string) error {
	_, err := r.db.Exec(
		`UPDATE videos SET width=$1, height=$2, duration=$3, codec=$4, updated_at=$5 WHERE id=$6`,
		width, height, duration, codec, time.Now(), id,
	)
	return err
}

func (r *VideoRepo) UpdateHLSPath(id, hlsPath string) error {
	_, err := r.db.Exec(
		`UPDATE videos SET hls_path=$1, updated_at=$2 WHERE id=$3`,
		hlsPath, time.Now(), id,
	)
	return err
}

// Delete removes a video and all dependent rows (events, sessions, jobs) in a transaction.
func (r *VideoRepo) Delete(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete events for sessions belonging to this video.
	if _, err := tx.Exec(`
		DELETE FROM playback_events pe
		USING playback_sessions ps
		WHERE pe.session_id = ps.id AND ps.video_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM playback_sessions WHERE video_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM transcode_jobs WHERE video_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM videos WHERE id = $1`, id); err != nil {
		return err
	}

	return tx.Commit()
}
