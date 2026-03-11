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

const videoSelect = `
	SELECT v.id, v.title, v.status,
	       COALESCE(v.width,0), COALESCE(v.height,0),
	       COALESCE(v.duration,''), COALESCE(v.codec,''), COALESCE(v.hls_path,''),
	       (SELECT COUNT(*) FROM playback_sessions ps WHERE ps.video_id = v.id),
	       v.created_at, v.updated_at
	FROM videos v`

func scanVideo(s interface{ Scan(...any) error }) (*models.Video, error) {
	v := &models.Video{}
	err := s.Scan(&v.ID, &v.Title, &v.Status, &v.Width, &v.Height,
		&v.Duration, &v.Codec, &v.HLSPath, &v.PlayCount, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func (r *VideoRepo) GetByID(id string) (*models.Video, error) {
	row := r.db.QueryRow(videoSelect+` WHERE v.id = $1`, id)
	v, err := scanVideo(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return v, err
}

func (r *VideoRepo) List() ([]*models.Video, error) {
	rows, err := r.db.Query(videoSelect + ` ORDER BY v.created_at DESC`)
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
