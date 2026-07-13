-- +goose Up
ALTER TABLE stock_sr_zone_analyses ADD COLUMN explanation TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_zones ADD COLUMN explanation TEXT NOT NULL DEFAULT 'null';

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN explanation;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN explanation;
