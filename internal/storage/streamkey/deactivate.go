package streamkey

import "context"

// Deactivate marks a stream key inactive, scoped to the owner.
func (r *Repo) Deactivate(ctx context.Context, id, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE stream_keys SET is_active = FALSE WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}
