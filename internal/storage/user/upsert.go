package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"philos-video/internal/models"
)

// UpsertFromGoogle inserts a new user or updates email/name/picture if the
// google_sub already exists. Returns the user and whether it was newly created.
func (r *Repo) UpsertFromGoogle(ctx context.Context, googleSub, email, name, picture string, defaultQuota int64) (*models.User, bool, error) {
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
