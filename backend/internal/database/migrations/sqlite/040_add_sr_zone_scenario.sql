-- +goose Up
ALTER TABLE stock_sr_zone_analyses ADD COLUMN scenario TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_zones ADD COLUMN scenario TEXT NOT NULL DEFAULT 'null';

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN scenario;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN scenario;
