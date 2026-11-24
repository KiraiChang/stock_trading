-- +goose Up
-- 訊號評估用的時間框架（I-140）：`1m`（盤中）、`1d`（盤前、收盤、手動評估預設）。
-- 欄位說明見 postgres 版與 docs/database-schema.md。
ALTER TABLE signals ADD COLUMN timeframe TEXT;

-- +goose Down
ALTER TABLE signals DROP COLUMN timeframe;
