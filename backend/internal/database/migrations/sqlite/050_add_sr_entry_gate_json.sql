-- +goose Up
ALTER TABLE stock_sr_decisions ADD COLUMN entry_executability_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN entry_blocking_zone_json TEXT NOT NULL DEFAULT 'null';

-- +goose Down
ALTER TABLE stock_sr_decisions DROP COLUMN entry_blocking_zone_json;
ALTER TABLE stock_sr_decisions DROP COLUMN entry_executability_json;
