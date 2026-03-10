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
