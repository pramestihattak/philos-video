package upload

import "context"

func (r *Repo) MarkChunkReceived(ctx context.Context, uploadID string, chunkNumber int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upload_chunks SET received=true WHERE upload_id=$1 AND chunk_number=$2`,
		uploadID, chunkNumber,
	)
	return err
}
