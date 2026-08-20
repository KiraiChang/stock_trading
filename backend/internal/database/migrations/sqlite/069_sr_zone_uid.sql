-- 分析快照 ↔ zone 身分的 join 路徑（T-048 階段 E）。
-- 完整設計理由見 postgres 版的註解與 docs/database-schema.md。
--
-- sqlite 差異：TEXT 沒有長度上限；ADD COLUMN 沒有 IF NOT EXISTS。
-- 可空且不加外鍵的理由與 postgres 版相同。

-- +goose Up
ALTER TABLE stock_sr_zones
    ADD COLUMN zone_uid TEXT;

CREATE INDEX IF NOT EXISTS idx_stock_sr_zones_zone_uid
    ON stock_sr_zones (zone_uid);

-- +goose Down
DROP INDEX IF EXISTS idx_stock_sr_zones_zone_uid;
ALTER TABLE stock_sr_zones DROP COLUMN zone_uid;
