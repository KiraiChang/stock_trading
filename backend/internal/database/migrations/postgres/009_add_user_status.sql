-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'inactive';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS status;
