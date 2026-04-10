-- +goose Up

CREATE TABLE IF NOT EXISTS comments (
    id         TEXT        PRIMARY KEY,
    video_id   TEXT        NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id    TEXT        NOT NULL REFERENCES users(id),
    body       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_comments_video_created ON comments(video_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_comments_user ON comments(user_id);

CREATE TABLE IF NOT EXISTS live_chat_messages (
    id         TEXT        PRIMARY KEY,
    stream_id  TEXT        NOT NULL,
    user_id    TEXT        NOT NULL REFERENCES users(id),
    body       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_live_chat_stream_created ON live_chat_messages(stream_id, created_at);

-- +goose Down

DROP TABLE IF EXISTS live_chat_messages;
DROP TABLE IF EXISTS comments;
