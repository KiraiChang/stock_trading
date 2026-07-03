-- +goose Up
-- touch_count 語意拆分（見 docs/sr-zone-scoring.md「十五」/Python
-- scoring.py::score_zone 說明）：touch_count 維持兩個方向加總（zone 整體
-- 活躍度），新增 support_touch_count/resistance_touch_count 分開統計，
-- 讓「作為支撐」跟「作為壓力」各自的歷史樣本數可以被診斷；confidence 也
-- 改成依角色只用其中一個方向的樣本數/穩定度計算，不會被另一個方向稀釋
-- 或拉抬。DEFAULT 0 只是為了讓既有資料列（若有）能通過 NOT NULL；新分析
-- 一律由 Go ToStore() 帶入正確值。
ALTER TABLE stock_sr_zones ADD COLUMN support_touch_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE stock_sr_zones ADD COLUMN resistance_touch_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN support_touch_count;
ALTER TABLE stock_sr_zones DROP COLUMN resistance_touch_count;
