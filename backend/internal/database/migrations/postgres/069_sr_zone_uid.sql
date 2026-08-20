-- 分析快照 ↔ zone 身分的 join 路徑（T-048 階段 E）。
-- 現況規格見 docs/database-schema.md 與 docs/sr-zone-scoring.md「Zone 身分與 ZoneMatcher」。
--
-- 要解的問題：階段 B 之後身分算得出來也存得起來（zone_instances），但「這次分析的第 N 個
-- zone 是哪個身分」只活在 Go handler 記憶體的 UIDByZoneKey 裡，DB 沒有任何 join 路徑。
-- 於是 T-041 的 timeline 與 T-049 的下游都得靠價格邊界回推——那正是本筆要消滅的模式。
--
-- **可空，而且刻意不加外鍵**：zone_instances 的寫入（Apply）在 stock_sr_zones 之後、
-- 且不同交易。加外鍵會讓「zones 已寫入、身分寫入失敗」這個既有的可容忍降級
-- （handler 只記 warn、分析照常回 201）直接違反約束，把降級升級成整筆分析失敗。
--
-- NULL 有三種語意，都不代表「這個 zone 沒有身分」：
--   1. 該次分析早於本 migration。
--   2. 當次身分比對或寫入降級了（見 sr_zones.go 的 matchZoneIdentity）。
--   3. 由 analysis/sr_analysis_provider.go 這條不做身分追蹤的路徑建立
--      （signal / 排程用，與 /sr-zones handler 是兩條路）。
--
-- 不回填既有列：回填要解的正是「兩個舊 zone_key 是不是同一個 zone」，
-- 而那是 T-048 本身要建的能力——回填不可能做對。

-- +goose Up
ALTER TABLE stock_sr_zones
    ADD COLUMN IF NOT EXISTS zone_uid VARCHAR(64) NULL;

CREATE INDEX IF NOT EXISTS idx_stock_sr_zones_zone_uid
    ON stock_sr_zones (zone_uid);

-- +goose Down
DROP INDEX IF EXISTS idx_stock_sr_zones_zone_uid;
ALTER TABLE stock_sr_zones DROP COLUMN IF EXISTS zone_uid;
