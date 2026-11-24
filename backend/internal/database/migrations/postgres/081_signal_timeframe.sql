-- +goose Up
-- 訊號評估用的時間框架（I-140）：`1m`（盤中）、`1d`（盤前、收盤、手動評估預設）。
-- 盤中 1 分 K 的 SUPPORT_BOUNCE 去重（B 規則）只看 timeframe = '1m' 的列，盤中與日 K 的判重歷史因此分開。
-- 可為 NULL、沒有預設值：NULL＝I-140 之前寫入的列（時間框架不明），舊資料⛔ 不回填。現況見 docs/database-schema.md 的 signals。
ALTER TABLE signals ADD COLUMN timeframe VARCHAR(8);

-- +goose Down
ALTER TABLE signals DROP COLUMN timeframe;
