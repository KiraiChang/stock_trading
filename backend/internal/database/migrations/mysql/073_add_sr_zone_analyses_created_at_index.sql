-- stock_sr_zone_analyses 的「最近 N 天」查詢索引。
-- 完整設計理由見 postgres 版的註解。
--
-- mysql 差異：CREATE INDEX / DROP INDEX 沒有 IF NOT EXISTS / IF EXISTS，
-- 且 DROP INDEX 要帶 ON <table>。索引欄位的 DESC 在 8.0 才真正生效。
-- **本 engine 從未部署過**（見 docs/issue.md I-054），只由
-- scripts/test-mysql-migrations.sh 驗證 DDL。

-- **索引帶 id DESC**：created_at 只有秒級精度，同一輪分析的多檔常落在同一秒。
-- 查詢用 ORDER BY created_at DESC, id DESC 讓同秒的那批有確定順序（撞到 limit 時
-- 邊界不會漂移），索引跟著帶 id 才走得到完整排序、不必再做一次 sort。

-- +goose Up
CREATE INDEX idx_stock_sr_zone_analyses_created_at
    ON stock_sr_zone_analyses (created_at DESC, id DESC);

-- +goose Down
DROP INDEX idx_stock_sr_zone_analyses_created_at ON stock_sr_zone_analyses;
