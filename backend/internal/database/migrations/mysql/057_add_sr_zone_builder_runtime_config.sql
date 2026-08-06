-- +goose Up
-- zone_builder_runtime_config 記錄這次分析實際用了哪組 zone builder 設定
-- （adaptive 是否啟用、落在哪個波動 bucket、reason_code）。Python pipeline 早就回傳，
-- 但 Go 端沒有欄位承接就被丟掉，前端因此看不到——見 docs/sr-zone-scoring.md
-- 的「Adaptive builder 選用與 zone_builder_runtime_config」。
-- 舊資料沒有這項紀錄，backfill 成 JSON 'null'（不是 SQL NULL）：store.RawJSON 是
-- 純 string、沒有實作 sql.Scanner，SQL NULL 會讓 scan 直接失敗。
ALTER TABLE stock_sr_zone_analyses ADD COLUMN zone_builder_runtime_config LONGTEXT NULL;
UPDATE stock_sr_zone_analyses SET zone_builder_runtime_config = 'null' WHERE zone_builder_runtime_config IS NULL;
ALTER TABLE stock_sr_zone_analyses MODIFY zone_builder_runtime_config LONGTEXT NOT NULL;

-- +goose Down
ALTER TABLE stock_sr_zone_analyses DROP COLUMN zone_builder_runtime_config;
