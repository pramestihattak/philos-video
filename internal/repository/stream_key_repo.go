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

func (r *StreamKeyRepo) Create(label string) (*models.StreamKey, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating stream key id: %w", err)
	}
	id := "sk_" + hex.EncodeToString(b)

	sk := &models.StreamKey{}
	err := r.db.QueryRow(
		`INSERT INTO stream_keys (id, user_label) VALUES ($1, $2)
		 RETURNING id, user_label, is_active, created_at`,
		id, label,
	).Scan(&sk.ID, &sk.UserLabel, &sk.IsActive, &sk.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating stream key: %w", err)
	}
	return sk, nil
}

func (r *StreamKeyRepo) GetByID(id string) (*models.StreamKey, error) {
	sk := &models.StreamKey{}
	err := r.db.QueryRow(
		`SELECT id, user_label, is_active, created_at FROM stream_keys WHERE id = $1`, id,
	).Scan(&sk.ID, &sk.UserLabel, &sk.IsActive, &sk.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting stream key: %w", err)
	}
	return sk, nil
}

func (r *StreamKeyRepo) List() ([]*models.StreamKey, error) {
	rows, err := r.db.Query(
		`SELECT id, user_label, is_active, created_at FROM stream_keys ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing stream keys: %w", err)
	}
	defer rows.Close()

	var keys []*models.StreamKey
	for rows.Next() {
		sk := &models.StreamKey{}
		if err := rows.Scan(&sk.ID, &sk.UserLabel, &sk.IsActive, &sk.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, sk)
	}
	return keys, rows.Err()
}

func (r *StreamKeyRepo) Deactivate(id string) error {
	_, err := r.db.Exec(`UPDATE stream_keys SET is_active = FALSE WHERE id = $1`, id)
	return err
}
