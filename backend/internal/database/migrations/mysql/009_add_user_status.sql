-- +goose Up
ALTER TABLE users ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'inactive';

-- +goose Down
ALTER TABLE users DROP COLUMN status;
