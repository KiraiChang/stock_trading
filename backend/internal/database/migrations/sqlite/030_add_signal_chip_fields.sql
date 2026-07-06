-- +goose Up
ALTER TABLE signals ADD COLUMN strength REAL NOT NULL DEFAULT 1.0;
ALTER TABLE signals ADD COLUMN chip_signal TEXT;

-- +goose Down
ALTER TABLE signals DROP COLUMN strength;
ALTER TABLE signals DROP COLUMN chip_signal;
