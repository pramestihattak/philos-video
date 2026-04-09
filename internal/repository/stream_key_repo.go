package repository

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	"philos-video/internal/models"
)

type StreamKeyRepo struct {
	db *sql.DB
}

func NewStreamKeyRepo(db *sql.DB) *StreamKeyRepo {
	return &StreamKeyRepo{db: db}
}

func (r *StreamKeyRepo) Create(label string, recordVOD bool, userID string) (*models.StreamKey, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating stream key id: %w", err)
	}
	id := "sk_" + hex.EncodeToString(b)

	sk := &models.StreamKey{}
	err := r.db.QueryRow(
		`INSERT INTO stream_keys (id, user_id, user_label, record_vod) VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, user_label, is_active, record_vod, created_at`,
		id, userID, label, recordVOD,
	).Scan(&sk.ID, &sk.UserID, &sk.UserLabel, &sk.IsActive, &sk.RecordVOD, &sk.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating stream key: %w", err)
	}
	return sk, nil
}

// GetByID is intentionally unscoped — used by RTMP ingest which only has the key secret.
func (r *StreamKeyRepo) GetByID(id string) (*models.StreamKey, error) {
	sk := &models.StreamKey{}
	err := r.db.QueryRow(
		`SELECT id, user_id, user_label, is_active, record_vod, created_at FROM stream_keys WHERE id = $1`, id,
	).Scan(&sk.ID, &sk.UserID, &sk.UserLabel, &sk.IsActive, &sk.RecordVOD, &sk.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting stream key: %w", err)
	}
	return sk, nil
}

// List returns all active stream keys for a specific user.
func (r *StreamKeyRepo) List(userID string) ([]*models.StreamKey, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, user_label, is_active, record_vod, created_at
		 FROM stream_keys WHERE user_id = $1 AND is_active = TRUE ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing stream keys: %w", err)
	}
	defer rows.Close()

	var keys []*models.StreamKey
	for rows.Next() {
		sk := &models.StreamKey{}
		if err := rows.Scan(&sk.ID, &sk.UserID, &sk.UserLabel, &sk.IsActive, &sk.RecordVOD, &sk.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, sk)
	}
	return keys, rows.Err()
}

// Deactivate marks a stream key inactive, scoped to the owner.
func (r *StreamKeyRepo) Deactivate(id, userID string) error {
	_, err := r.db.Exec(
		`UPDATE stream_keys SET is_active = FALSE WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

// UpdateRecordVOD changes the record_vod flag, scoped to the owner.
func (r *StreamKeyRepo) UpdateRecordVOD(id string, recordVOD bool, userID string) error {
	_, err := r.db.Exec(
		`UPDATE stream_keys SET record_vod = $1 WHERE id = $2 AND user_id = $3`,
		recordVOD, id, userID,
	)
	return err
}
