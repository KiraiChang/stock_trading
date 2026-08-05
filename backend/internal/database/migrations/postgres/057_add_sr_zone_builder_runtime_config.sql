-- +goose Up
-- 見 mysql/057 的說明。postgres 的其他 SR zone JSON 欄位在 046 已轉成 JSONB，
-- 這裡直接以 JSONB 建立並沿用 'null'::jsonb 的預設值，讓舊資料等同「無紀錄」。
ALTER TABLE stock_sr_zone_analyses
    ADD COLUMN IF NOT EXISTS zone_builder_runtime_config JSONB NOT NULL DEFAULT 'null'::jsonb;

-- +goose Down
ALTER TABLE stock_sr_zone_analyses DROP COLUMN IF EXISTS zone_builder_runtime_config;
