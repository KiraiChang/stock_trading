-- +goose Up
-- 見 mysql/057 的說明。sqlite 的 ALTER TABLE ADD COLUMN 支援常數 DEFAULT，
-- 所以一步就能建成 NOT NULL，舊資料自動填 JSON 'null'。
ALTER TABLE stock_sr_zone_analyses ADD COLUMN zone_builder_runtime_config TEXT NOT NULL DEFAULT 'null';

-- +goose Down
ALTER TABLE stock_sr_zone_analyses DROP COLUMN zone_builder_runtime_config;
