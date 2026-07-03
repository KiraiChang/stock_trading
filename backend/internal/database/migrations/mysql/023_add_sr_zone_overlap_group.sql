-- +goose Up
-- 跨方法重疊分群，見 sqlite 版本同名 migration 的說明。
ALTER TABLE stock_sr_zones ADD COLUMN overlap_group INT;
ALTER TABLE stock_sr_zones ADD COLUMN confluence_count INT NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN overlap_group;
ALTER TABLE stock_sr_zones DROP COLUMN confluence_count;
