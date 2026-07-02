-- +goose Up
ALTER TABLE watchlists ADD COLUMN watched TINYINT(1) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE watchlists DROP COLUMN watched;
