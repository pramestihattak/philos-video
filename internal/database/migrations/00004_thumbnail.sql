-- +goose Up
ALTER TABLE videos ADD COLUMN IF NOT EXISTS thumbnail_path TEXT;

-- +goose Down
ALTER TABLE videos DROP COLUMN IF EXISTS thumbnail_path;
