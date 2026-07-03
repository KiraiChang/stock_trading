-- +goose Up
-- 模型設定可追溯性，見 sqlite 版本同名 migration 的說明。
ALTER TABLE stock_sr_zone_analyses ADD COLUMN model_config_hash VARCHAR(20) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE stock_sr_zone_analyses DROP COLUMN model_config_hash;
