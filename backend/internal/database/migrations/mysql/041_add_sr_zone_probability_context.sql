-- +goose Up
ALTER TABLE stock_sr_zone_analyses ADD COLUMN probability_context LONGTEXT NULL;
ALTER TABLE stock_sr_zones ADD COLUMN probability_context LONGTEXT NULL;
UPDATE stock_sr_zone_analyses SET probability_context = 'null' WHERE probability_context IS NULL;
UPDATE stock_sr_zones SET probability_context = 'null' WHERE probability_context IS NULL;
ALTER TABLE stock_sr_zone_analyses MODIFY probability_context LONGTEXT NOT NULL;
ALTER TABLE stock_sr_zones MODIFY probability_context LONGTEXT NOT NULL;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN probability_context;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN probability_context;
