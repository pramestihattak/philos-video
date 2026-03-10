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

func (r *VideoRepo) GetByID(id string) (*models.Video, error) {
	v := &models.Video{}
	err := r.db.QueryRow(
		`SELECT id, title, status,
		        COALESCE(width,0), COALESCE(height,0),
		        COALESCE(duration,''), COALESCE(codec,''), COALESCE(hls_path,''),
		        created_at, updated_at
		 FROM videos WHERE id = $1`, id,
	).Scan(&v.ID, &v.Title, &v.Status, &v.Width, &v.Height,
		&v.Duration, &v.Codec, &v.HLSPath, &v.CreatedAt, &v.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return v, err
}

func (r *VideoRepo) List() ([]*models.Video, error) {
	rows, err := r.db.Query(
		`SELECT id, title, status,
		        COALESCE(width,0), COALESCE(height,0),
		        COALESCE(duration,''), COALESCE(codec,''), COALESCE(hls_path,''),
		        created_at, updated_at
		 FROM videos ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		v := &models.Video{}
		if err := rows.Scan(&v.ID, &v.Title, &v.Status, &v.Width, &v.Height,
			&v.Duration, &v.Codec, &v.HLSPath, &v.CreatedAt, &v.UpdatedAt); err != nil {
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
