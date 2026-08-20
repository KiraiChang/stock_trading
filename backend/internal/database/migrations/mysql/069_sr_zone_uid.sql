-- 分析快照 ↔ zone 身分的 join 路徑（T-048 階段 E）。
-- 完整設計理由見 postgres 版的註解與 docs/database-schema.md。
--
-- mysql 差異：ADD COLUMN / DROP INDEX 沒有 IF NOT EXISTS / IF EXISTS。
-- **本 engine 從未部署過**（見 docs/issue.md I-054），只由
-- scripts/test-mysql-migrations.sh 驗證 DDL。

-- +goose Up
ALTER TABLE stock_sr_zones
    ADD COLUMN zone_uid VARCHAR(64) NULL;

CREATE INDEX idx_stock_sr_zones_zone_uid
    ON stock_sr_zones (zone_uid);

-- +goose Down
DROP INDEX idx_stock_sr_zones_zone_uid ON stock_sr_zones;
ALTER TABLE stock_sr_zones DROP COLUMN zone_uid;
