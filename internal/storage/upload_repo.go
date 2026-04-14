package storage

import (
	"context"
	"database/sql"
)

type UploadRepo struct {
	db *sql.DB
}

func NewUploadRepo(db *sql.DB) *UploadRepo {
	return &UploadRepo{db: db}
}

func (r *UploadRepo) CreateChunks(ctx context.Context, uploadID string, totalChunks int) error {
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

func (r *UploadRepo) MarkChunkReceived(ctx context.Context, uploadID string, chunkNumber int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE upload_chunks SET received=true WHERE upload_id=$1 AND chunk_number=$2`,
		uploadID, chunkNumber,
	)
	return err
}

func (r *UploadRepo) GetProgress(ctx context.Context, uploadID string) (received, total int, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FILTER (WHERE received), COUNT(*) FROM upload_chunks WHERE upload_id=$1`,
		uploadID,
	).Scan(&received, &total)
	return
}
