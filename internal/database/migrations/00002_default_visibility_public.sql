-- +goose Up
ALTER TABLE videos ALTER COLUMN visibility SET DEFAULT 'public';

-- +goose Down
ALTER TABLE videos ALTER COLUMN visibility SET DEFAULT 'private';
