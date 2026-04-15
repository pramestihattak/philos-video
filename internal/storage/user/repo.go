package user

import (
	"database/sql"

	"philos-video/internal/models"
)

// Repo is the PostgreSQL implementation of Repository.
type Repo struct {
	db *sql.DB
}

// New creates a user Repo.
func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

const userCols = `id, google_sub, email, name, picture, upload_quota_bytes, used_bytes, created_at`

func scanUser(row interface{ Scan(...any) error }) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(
		&u.ID, &u.GoogleSub, &u.Email, &u.Name, &u.Picture,
		&u.UploadQuotaBytes, &u.UsedBytes, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}
