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
