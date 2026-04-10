package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"philos-video/internal/models"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

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

const userCols = `id, google_sub, email, name, picture, upload_quota_bytes, used_bytes, created_at`

// UpsertFromGoogle inserts a new user or updates email/name/picture if the
// google_sub already exists. Returns the user and whether it was newly created.
func (r *UserRepo) UpsertFromGoogle(ctx context.Context, googleSub, email, name, picture string, defaultQuota int64) (*models.User, bool, error) {
	id := uuid.New().String()

	row := r.db.QueryRowContext(ctx,
		`INSERT INTO users (id, google_sub, email, name, picture, upload_quota_bytes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (google_sub) DO UPDATE
		   SET email = EXCLUDED.email,
		       name  = EXCLUDED.name,
		       picture = EXCLUDED.picture
		 RETURNING `+userCols+`, (xmax = 0) AS inserted`,
		id, googleSub, email, name, picture, defaultQuota,
	)

	u := &models.User{}
	var inserted bool
	err := row.Scan(
		&u.ID, &u.GoogleSub, &u.Email, &u.Name, &u.Picture,
		&u.UploadQuotaBytes, &u.UsedBytes, &u.CreatedAt, &inserted,
	)
	if err != nil {
		return nil, false, fmt.Errorf("upserting user: %w", err)
	}
	return u, inserted, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE id = $1`, id,
	)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return u, nil
}

// HasQuotaFor returns true if the user has enough quota to upload size bytes.
// A quota of 0 means unlimited.
func (r *UserRepo) HasQuotaFor(ctx context.Context, userID string, size int64) (bool, error) {
	var quota, used int64
	err := r.db.QueryRowContext(ctx,
		`SELECT upload_quota_bytes, used_bytes FROM users WHERE id = $1`, userID,
	).Scan(&quota, &used)
	if err != nil {
		return false, fmt.Errorf("checking quota: %w", err)
	}
	if quota == 0 {
		return true, nil
	}
	return used+size <= quota, nil
}

// IncUsedBytes atomically adds delta to the user's used_bytes counter.
func (r *UserRepo) IncUsedBytes(ctx context.Context, userID string, delta int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET used_bytes = used_bytes + $1 WHERE id = $2`, delta, userID,
	)
	return err
}

// DecUsedBytes atomically subtracts delta from the user's used_bytes counter
// (clamped to zero to avoid negatives from race conditions).
func (r *UserRepo) DecUsedBytes(ctx context.Context, userID string, delta int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET used_bytes = GREATEST(0, used_bytes - $1) WHERE id = $2`, delta, userID,
	)
	return err
}
