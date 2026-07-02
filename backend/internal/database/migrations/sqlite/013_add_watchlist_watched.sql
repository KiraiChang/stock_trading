-- +goose Up
ALTER TABLE watchlists ADD COLUMN watched INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; leave as-is
