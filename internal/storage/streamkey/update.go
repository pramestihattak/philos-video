package streamkey

import "context"

// UpdateRecordVOD changes the record_vod flag, scoped to the owner.
func (r *Repo) UpdateRecordVOD(ctx context.Context, id string, recordVOD bool, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE stream_keys SET record_vod = $1 WHERE id = $2 AND user_id = $3`,
		recordVOD, id, userID,
	)
	return err
}
