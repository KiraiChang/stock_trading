-- +goose Up
ALTER TABLE stock_sr_zone_analyses ADD COLUMN pipeline_version TEXT NOT NULL DEFAULT '';
ALTER TABLE stock_sr_zone_analyses ADD COLUMN evidence TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_zones ADD COLUMN features TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_zones ADD COLUMN evidence TEXT NOT NULL DEFAULT 'null';

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN evidence;
ALTER TABLE stock_sr_zones DROP COLUMN features;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN evidence;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN pipeline_version;
