package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

const migrationSQL = `
CREATE TABLE IF NOT EXISTS videos (
    id         TEXT        PRIMARY KEY,
    title      TEXT        NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'uploading',
    width      INT,
    height     INT,
    duration   TEXT,
    codec      TEXT,
    hls_path   TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS upload_chunks (
    upload_id    TEXT    NOT NULL,
    chunk_number INT     NOT NULL,
    received     BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (upload_id, chunk_number)
);

CREATE TABLE IF NOT EXISTS transcode_jobs (
    id         TEXT             PRIMARY KEY,
    video_id   TEXT             NOT NULL REFERENCES videos(id),
    status     TEXT             NOT NULL DEFAULT 'queued',
    stage      TEXT,
    progress   DOUBLE PRECISION NOT NULL DEFAULT 0,
    error      TEXT,
    created_at TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);
`

func Connect(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return db, nil
}

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(migrationSQL); err != nil {
		return fmt.Errorf("running migration: %w", err)
	}
	return nil
}
