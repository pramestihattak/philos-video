package user

import (
	"context"
	"database/sql"
	"fmt"

	"philos-video/internal/models"
)

func (r *Repo) GetByID(ctx context.Context, id string) (*models.User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return u, nil
}
