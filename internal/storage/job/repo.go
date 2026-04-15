package job

import "database/sql"

// Repo is the PostgreSQL implementation of Repository.
type Repo struct {
	db *sql.DB
}

// New creates a job Repo.
func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

const jobCols = `id, video_id, status, COALESCE(stage,''), progress, COALESCE(error,''), created_at, updated_at`
