-- +goose Up
ALTER TABLE stock_sr_zone_analyses ADD COLUMN probability_context TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_zones ADD COLUMN probability_context TEXT NOT NULL DEFAULT 'null';

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN probability_context;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN probability_context;
