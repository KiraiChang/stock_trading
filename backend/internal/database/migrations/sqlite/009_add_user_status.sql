-- +goose Up
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'inactive';

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; leave as-is
