-- +goose Up
-- 跨方法重疊分群（confluence，見 docs/sr-zone-scoring.md「十六」/Python
-- scoring.py::_group_overlapping_zones 說明）：不同方法（ATR/
-- volume_profile）各自建出來、但實際上指向同一價位帶的 zone 會有相同的
-- overlap_group，confluence_count 是這個群組裡的 zone 數（單獨一個 zone
-- 沒有群組，overlap_group 為 NULL、confluence_count=1）。不合併/刪除任何
-- zone，只標記供前端顯示「多方法共振」。
ALTER TABLE stock_sr_zones ADD COLUMN overlap_group INTEGER;
ALTER TABLE stock_sr_zones ADD COLUMN confluence_count INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE stock_sr_zones DROP COLUMN overlap_group;
ALTER TABLE stock_sr_zones DROP COLUMN confluence_count;
