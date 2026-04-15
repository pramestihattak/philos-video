package user

import (
	"context"
	"fmt"
)

// HasQuotaFor returns true if the user has enough quota to upload size bytes.
// A quota of 0 means unlimited.
func (r *Repo) HasQuotaFor(ctx context.Context, userID string, size int64) (bool, error) {
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
func (r *Repo) IncUsedBytes(ctx context.Context, userID string, delta int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET used_bytes = used_bytes + $1 WHERE id = $2`, delta, userID,
	)
	return err
}

// DecUsedBytes atomically subtracts delta from the user's used_bytes counter
// (clamped to zero to avoid negatives from race conditions).
func (r *Repo) DecUsedBytes(ctx context.Context, userID string, delta int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET used_bytes = GREATEST(0, used_bytes - $1) WHERE id = $2`, delta, userID,
	)
	return err
}
