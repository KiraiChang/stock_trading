-- +goose Up
ALTER TABLE signals ADD COLUMN strength NUMERIC(5,4) NOT NULL DEFAULT 1.0;
ALTER TABLE signals ADD COLUMN chip_signal VARCHAR(20) NULL;

-- +goose Down
ALTER TABLE signals DROP COLUMN strength;
ALTER TABLE signals DROP COLUMN chip_signal;
