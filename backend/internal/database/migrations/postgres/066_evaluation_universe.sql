-- 評估標的池（T-040 Step 5 / Phase 2）。規格見
-- docs/evaluation-universe-selection-plan.md 的「Step 5 執行計畫書」。
--
-- 這張表**與 watchlists 分離**：watchlists 驅動盤中掃描、籌碼同步、日結掃描、signal 與
-- production SR 分析六個流程，把 131 檔塞進去會讓每一個都乘上約 12 倍。本表只有一個用途——
-- 每日盤後更新這批標的的日 K，讓歷史持續累積供 T-002 / T-003 研究使用。
--
-- **不參與任何交易決策或狀態推導。**
--
-- bucket_edge_low / bucket_edge_high 是刻意的反正規化：bucket_hint 單獨存在無法回答
-- 「這個 bucket 是用哪組邊界判的」。實測 2026-08-17 有 3 檔 atr_pct 完全未變卻換桶，
-- 只因母體變了、分位數邊界移動。131 列的重複成本可忽略，換來每一列自我描述。
-- 兩個值應填入 zone_builder.py 的 LOW/HIGH_VOLATILITY_THRESHOLD 當下的值。

-- +goose Up
CREATE TABLE IF NOT EXISTS evaluation_universe (
    id               BIGSERIAL      PRIMARY KEY,
    symbol           VARCHAR(10)    NOT NULL UNIQUE,
    bucket_hint      VARCHAR(32)    NOT NULL,
    bucket_edge_low  DECIMAL(18,10) NOT NULL,
    bucket_edge_high DECIMAL(18,10) NOT NULL,
    universe_version VARCHAR(32)    NOT NULL,
    -- primary 參與股票 builder 決策；supplemental 只作交叉觀察（債券/槓桿 ETF、
    -- 以及分級保留下來的不合格 watchlist）。見計畫書「watchlist 的分級保留」。
    universe_role    VARCHAR(16)    NOT NULL DEFAULT 'primary',
    selected_at      TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    source           VARCHAR(64)    NOT NULL,
    -- false ＝ 保留紀錄但不再納入每日維護。刻意不用刪除：入池/退池的歷史本身是研究紀錄。
    active           BOOLEAN        NOT NULL DEFAULT TRUE,
    note             TEXT           NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_evaluation_universe_edges
        CHECK (bucket_edge_low > 0 AND bucket_edge_high > bucket_edge_low)
);
-- 每日排程只查 active 的成員，這是唯一的熱路徑。
CREATE INDEX IF NOT EXISTS idx_evaluation_universe_active
    ON evaluation_universe (active, symbol);

-- +goose Down
DROP INDEX IF EXISTS idx_evaluation_universe_active;
DROP TABLE IF EXISTS evaluation_universe;
