-- stock_sr_zone_analyses 的「最近 N 天」查詢索引。
-- 完整設計理由見 postgres 版的註解。
--
-- sqlite 差異：索引的 DESC 是支援的（3.8.3+），語法與 postgres 相同。

-- **索引帶 id DESC**：created_at 只有秒級精度，同一輪分析的多檔常落在同一秒。
-- 查詢用 ORDER BY created_at DESC, id DESC 讓同秒的那批有確定順序（撞到 limit 時
-- 邊界不會漂移），索引跟著帶 id 才走得到完整排序、不必再做一次 sort。

-- +goose Up
CREATE INDEX IF NOT EXISTS idx_stock_sr_zone_analyses_created_at
    ON stock_sr_zone_analyses (created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_stock_sr_zone_analyses_created_at;
