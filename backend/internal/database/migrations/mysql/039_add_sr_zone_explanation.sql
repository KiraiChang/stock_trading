-- +goose Up
ALTER TABLE stock_sr_zone_analyses ADD COLUMN explanation LONGTEXT NULL;
ALTER TABLE stock_sr_zones ADD COLUMN explanation LONGTEXT NULL;
UPDATE stock_sr_zone_analyses SET explanation = 'null' WHERE explanation IS NULL;
UPDATE stock_sr_zones SET explanation = 'null' WHERE explanation IS NULL;
ALTER TABLE stock_sr_zone_analyses MODIFY explanation LONGTEXT NOT NULL;
ALTER TABLE stock_sr_zones MODIFY explanation LONGTEXT NOT NULL;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN explanation;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN explanation;
