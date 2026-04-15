package upload

import "context"

func (r *Repo) GetProgress(ctx context.Context, uploadID string) (received, total int, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FILTER (WHERE received), COUNT(*) FROM upload_chunks WHERE upload_id=$1`,
		uploadID,
	).Scan(&received, &total)
	return
}
