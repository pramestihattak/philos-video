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

CREATE TABLE IF NOT EXISTS playback_sessions (
    id             TEXT        PRIMARY KEY,
    video_id       TEXT        NOT NULL REFERENCES videos(id),
    token          TEXT        NOT NULL,
    device_type    TEXT,
    user_agent     TEXT,
    ip_address     TEXT,
    started_at     TIMESTAMPTZ DEFAULT NOW(),
    last_active_at TIMESTAMPTZ DEFAULT NOW(),
    ended_at       TIMESTAMPTZ,
    status         TEXT        DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS playback_events (
    id                   BIGSERIAL   PRIMARY KEY,
    session_id           TEXT        NOT NULL REFERENCES playback_sessions(id),
    video_id             TEXT        NOT NULL,
    event_type           TEXT        NOT NULL,
    timestamp            TIMESTAMPTZ DEFAULT NOW(),
    segment_number       INTEGER,
    segment_quality      TEXT,
    segment_bytes        BIGINT,
    download_time_ms     INTEGER,
    throughput_bps       BIGINT,
    current_quality      TEXT,
    buffer_length        DOUBLE PRECISION,
    playback_position    DOUBLE PRECISION,
    rebuffer_duration_ms INTEGER,
    quality_from         TEXT,
    quality_to           TEXT,
    error_code           TEXT,
    error_message        TEXT
);

CREATE INDEX IF NOT EXISTS idx_playback_events_session   ON playback_events(session_id);
CREATE INDEX IF NOT EXISTS idx_playback_events_type_time ON playback_events(event_type, timestamp);
CREATE INDEX IF NOT EXISTS idx_playback_events_video     ON playback_events(video_id, timestamp);

CREATE TABLE IF NOT EXISTS stream_keys (
    id         TEXT        PRIMARY KEY,
    user_label TEXT        NOT NULL,
    is_active  BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS live_streams (
    id            TEXT        PRIMARY KEY,
    stream_key_id TEXT        NOT NULL REFERENCES stream_keys(id),
    title         TEXT        NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'waiting',
    source_width  INT,
    source_height INT,
    source_codec  TEXT,
    source_fps    TEXT,
    hls_path      TEXT,
    video_id      TEXT,
    started_at    TIMESTAMPTZ,
    ended_at      TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_live_streams_status     ON live_streams(status);
CREATE INDEX IF NOT EXISTS idx_live_streams_stream_key ON live_streams(stream_key_id);

ALTER TABLE playback_sessions ADD COLUMN IF NOT EXISTS stream_id TEXT;
ALTER TABLE playback_sessions ALTER COLUMN video_id DROP NOT NULL;
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
