-- +goose Up
ALTER TABLE stock_sr_decisions ADD COLUMN entry_executability_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN entry_blocking_zone_json LONGTEXT NULL;

UPDATE stock_sr_decisions SET entry_executability_json = 'null' WHERE entry_executability_json IS NULL;
UPDATE stock_sr_decisions SET entry_blocking_zone_json = 'null' WHERE entry_blocking_zone_json IS NULL;

ALTER TABLE stock_sr_decisions MODIFY entry_executability_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY entry_blocking_zone_json LONGTEXT NOT NULL;

-- +goose Down
ALTER TABLE stock_sr_decisions DROP COLUMN entry_blocking_zone_json;
ALTER TABLE stock_sr_decisions DROP COLUMN entry_executability_json;
