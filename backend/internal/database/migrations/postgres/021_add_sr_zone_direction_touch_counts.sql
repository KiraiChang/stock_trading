-- +goose Up
-- touch_count 語意拆分，見 sqlite 版本同名 migration 的說明。
ALTER TABLE stock_sr_zones ADD COLUMN support_touch_count INT NOT NULL DEFAULT 0;
ALTER TABLE stock_sr_zones ADD COLUMN resistance_touch_count INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN support_touch_count;
ALTER TABLE stock_sr_zones DROP COLUMN resistance_touch_count;
