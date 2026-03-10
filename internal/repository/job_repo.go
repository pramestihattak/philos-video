package repository

import (
	"database/sql"
	"time"

	"philos-video/internal/models"
)

type JobRepo struct {
	db *sql.DB
}

func NewJobRepo(db *sql.DB) *JobRepo {
	return &JobRepo{db: db}
}

func (r *JobRepo) Create(job *models.TranscodeJob) error {
	_, err := r.db.Exec(
		`INSERT INTO transcode_jobs (id, video_id, status) VALUES ($1, $2, $3)`,
		job.ID, job.VideoID, job.Status,
	)
	return err
}

func (r *JobRepo) GetByID(id string) (*models.TranscodeJob, error) {
	job := &models.TranscodeJob{}
	err := r.db.QueryRow(
		`SELECT id, video_id, status, COALESCE(stage,''), progress, COALESCE(error,''),
		        created_at, updated_at
		 FROM transcode_jobs WHERE id=$1`, id,
	).Scan(&job.ID, &job.VideoID, &job.Status, &job.Stage, &job.Progress, &job.Error,
		&job.CreatedAt, &job.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return job, err
}

func (r *JobRepo) GetByVideoID(videoID string) (*models.TranscodeJob, error) {
	job := &models.TranscodeJob{}
	err := r.db.QueryRow(
		`SELECT id, video_id, status, COALESCE(stage,''), progress, COALESCE(error,''),
		        created_at, updated_at
		 FROM transcode_jobs WHERE video_id=$1 ORDER BY created_at DESC LIMIT 1`, videoID,
	).Scan(&job.ID, &job.VideoID, &job.Status, &job.Stage, &job.Progress, &job.Error,
		&job.CreatedAt, &job.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return job, err
}

func (r *JobRepo) UpdateRunning(id string) error {
	_, err := r.db.Exec(
		`UPDATE transcode_jobs SET status='running', updated_at=$1 WHERE id=$2`,
		time.Now(), id,
	)
	return err
}

func (r *JobRepo) UpdateProgress(id, stage string, progress float64) error {
	_, err := r.db.Exec(
		`UPDATE transcode_jobs SET stage=$1, progress=$2, updated_at=$3 WHERE id=$4`,
		stage, progress, time.Now(), id,
	)
	return err
}

func (r *JobRepo) Complete(id string) error {
	_, err := r.db.Exec(
		`UPDATE transcode_jobs SET status='completed', progress=1.0, updated_at=$1 WHERE id=$2`,
		time.Now(), id,
	)
	return err
}

func (r *JobRepo) Fail(id, errMsg string) error {
	_, err := r.db.Exec(
		`UPDATE transcode_jobs SET status='failed', error=$1, updated_at=$2 WHERE id=$3`,
		errMsg, time.Now(), id,
	)
	return err
}

func (r *JobRepo) ListQueued() ([]string, error) {
	rows, err := r.db.Query(
		`SELECT id FROM transcode_jobs WHERE status='queued' ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
