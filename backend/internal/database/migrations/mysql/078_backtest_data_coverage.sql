-- +goose Up
-- 回測結果的覆蓋範圍（I-137）：每一檔實際取到的首尾日期、根數、暖身根數、是否被略過，存成 JSON 陣列的文字。
-- 格式與判定見 docs/database-schema.md 的 backtest_results。NULL＝I-137 之前的結果（覆蓋範圍不明），舊資料⛔ 不回補。
-- 可為 NULL、沒有預設值：值由 python 寫入；python 兩個服務啟動時會等這個欄位出現才接任務。
ALTER TABLE backtest_results ADD COLUMN data_coverage TEXT;

-- +goose Down
ALTER TABLE backtest_results DROP COLUMN data_coverage;
