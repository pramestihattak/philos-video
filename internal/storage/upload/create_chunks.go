package upload

import "context"

func (r *Repo) CreateChunks(ctx context.Context, uploadID string, totalChunks int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO upload_chunks (upload_id, chunk_number, received) VALUES ($1, $2, false)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range totalChunks {
		if _, err := stmt.ExecContext(ctx, uploadID, i); err != nil {
			return err
		}
	}
	return tx.Commit()
}
