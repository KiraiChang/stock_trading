-- stock_sr_zone_analyses 的「最近 N 天」查詢索引。
-- 現況規格見 docs/architecture.md 的排程說明段。
--
-- 要解的問題：sr_zone_verify 原本取「最近 50 筆」分析來重驗 zone 有沒有被突破。
-- 那個數字是分析還沒排程化的年代訂的（一天 1~3 筆，50 筆約涵蓋 20 個交易日）；
-- watchlist 11 檔 × 每日兩輪之後一天就 22 筆，50 筆只剩約 2.3 個交易日，
-- watchlist 再擴大就不到一天。改成「最近 N 天」讓覆蓋窗口與 watchlist 大小脫鉤。
--
-- **既有的 idx_stock_sr_zone_analyses_symbol 撐不住**：它是 (symbol, created_at DESC)，
-- leading column 是 symbol，而排程那條查詢不帶 symbol（要全 watchlist 的分析），
-- 走不到它，只能全表掃描後排序。本索引就是支撐那句查詢的。
--
-- **用 created_at 而不是 analyzed_at**：analyzed_at 是日期粒度（同一交易日的 17:00
-- 與 22:00 兩輪都寫成當日 00:00，它是「今天這根 K 棒分析過沒」的判定依據），
-- 真正的寫入時間在 created_at。要「最近 N 天實際跑過的分析」只能用後者。

-- **索引帶 id DESC**：created_at 只有秒級精度，同一輪分析的多檔常落在同一秒。
-- 查詢用 ORDER BY created_at DESC, id DESC 讓同秒的那批有確定順序（撞到 limit 時
-- 邊界不會漂移），索引跟著帶 id 才走得到完整排序、不必再做一次 sort。

-- +goose Up
CREATE INDEX IF NOT EXISTS idx_stock_sr_zone_analyses_created_at
    ON stock_sr_zone_analyses (created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_stock_sr_zone_analyses_created_at;
