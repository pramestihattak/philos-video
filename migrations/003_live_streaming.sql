-- Migration 003: Live Streaming
-- This SQL is inlined in internal/database/postgres.go

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
