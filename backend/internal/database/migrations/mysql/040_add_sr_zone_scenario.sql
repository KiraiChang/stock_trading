-- +goose Up
ALTER TABLE stock_sr_zone_analyses ADD COLUMN scenario LONGTEXT NULL;
ALTER TABLE stock_sr_zones ADD COLUMN scenario LONGTEXT NULL;
UPDATE stock_sr_zone_analyses SET scenario = 'null' WHERE scenario IS NULL;
UPDATE stock_sr_zones SET scenario = 'null' WHERE scenario IS NULL;
ALTER TABLE stock_sr_zone_analyses MODIFY scenario LONGTEXT NOT NULL;
ALTER TABLE stock_sr_zones MODIFY scenario LONGTEXT NOT NULL;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN scenario;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN scenario;
