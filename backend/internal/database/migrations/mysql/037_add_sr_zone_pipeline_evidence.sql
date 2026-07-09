-- +goose Up
ALTER TABLE stock_sr_zone_analyses ADD COLUMN pipeline_version VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE stock_sr_zone_analyses ADD COLUMN evidence LONGTEXT NULL;
UPDATE stock_sr_zone_analyses SET evidence = 'null' WHERE evidence IS NULL;
ALTER TABLE stock_sr_zone_analyses MODIFY evidence LONGTEXT NOT NULL;
ALTER TABLE stock_sr_zones ADD COLUMN features LONGTEXT NULL;
ALTER TABLE stock_sr_zones ADD COLUMN evidence LONGTEXT NULL;
UPDATE stock_sr_zones SET features = 'null' WHERE features IS NULL;
UPDATE stock_sr_zones SET evidence = 'null' WHERE evidence IS NULL;
ALTER TABLE stock_sr_zones MODIFY features LONGTEXT NOT NULL;
ALTER TABLE stock_sr_zones MODIFY evidence LONGTEXT NOT NULL;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN evidence;
ALTER TABLE stock_sr_zones DROP COLUMN features;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN evidence;
ALTER TABLE stock_sr_zone_analyses DROP COLUMN pipeline_version;
