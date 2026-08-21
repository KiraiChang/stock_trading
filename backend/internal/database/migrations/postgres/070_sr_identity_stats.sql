-- 身分關聯決策的逐次分析統計（todo.md T-050）。
-- 現況規格見 docs/database-schema.md 與 docs/sr-zone-scoring.md「可觀測性」。
--
-- 要解的問題：這些計數目前只以**單筆結構化 log** 輸出。log 答得出「這一次分析發生了
-- 什麼」，但答不出**趨勢**——「alias 命中率從 2% 爬到 30%」代表 zone 邊界漂移在惡化，
-- 而那要靠人每天 grep 才看得到。
--
-- **刻意不沿用 job_runs 的保留策略。** job_runs 由 runPreMarket 每天 08:50
-- `DeleteBefore(TodayTaipei())` 只留當天——2026-08-21 實測：前一晚 22:00 那輪排程的紀錄
-- 早上就查不到了。那正是本表要解的問題，複製那個策略等於白做。
--
-- 量級：一次分析一列，排程上線後約 22 列/天 ≈ 8000 列/年，欄位幾乎都是整數。
--
-- **母體是分析的子集**：這些統計只在 reuse_existing=false 那條路徑產生
-- （portfolio/analyzer 的重用路徑不追身分，也就沒有統計）。analysis_id 因此是必要的：
-- 分母要 join stock_sr_zone_analyses 算出來，而不是靠讀者記得這條限制。

-- +goose Up
CREATE TABLE IF NOT EXISTS sr_identity_stats (
    id          BIGSERIAL   PRIMARY KEY,
    analysis_id BIGINT      NOT NULL,
    symbol      VARCHAR(10) NOT NULL,
    timeframe   VARCHAR(8)  NOT NULL,

    -- 三段關聯決策各自命中幾次（優先序見 sr-zone-scoring.md）。
    matched_by_chain   INTEGER NOT NULL DEFAULT 0,
    matched_by_current INTEGER NOT NULL DEFAULT 0,
    matched_by_alias   INTEGER NOT NULL DEFAULT 0,

    -- 以下都是「應該很少、變多就要看一眼」的計數。
    unmatched_keys       INTEGER NOT NULL DEFAULT 0,
    carried_noop         INTEGER NOT NULL DEFAULT 0,
    zone_ended_skipped   INTEGER NOT NULL DEFAULT 0,
    chain_conflicts      INTEGER NOT NULL DEFAULT 0,
    chain_key_ambiguous  INTEGER NOT NULL DEFAULT 0,
    alias_ambiguous      INTEGER NOT NULL DEFAULT 0,
    carried_parse_fail   INTEGER NOT NULL DEFAULT 0,

    -- **與其他欄位語意不同**：這是寫入前掃出來的終態矛盾，**必須恆為零**。
    -- 其他欄位問的是「分佈正不正常」，這一欄問的是「不變式有沒有被違反」，
    -- 兩者的告警語意完全不同，不要混進同一組比率裡。
    invariant_violations INTEGER NOT NULL DEFAULT 0,

    -- zone 側（T-048 階段 B）：目前只有 warn，同樣沒有趨勢。
    -- degraded 代表這次分析的 zone 身分比對整個沒跑成（取資料或比對失敗），
    -- 此時事件層也會跳過，其餘計數會全為 0——**別把它讀成「這次很乾淨」**。
    zone_identity_degraded  BOOLEAN NOT NULL DEFAULT FALSE,
    event_identity_degraded BOOLEAN NOT NULL DEFAULT FALSE,
    -- 這次進入 matcher 的既有身分數（ListLive 的結果數），是上面幾個比率的分母參考。
    zone_live_candidates INTEGER NOT NULL DEFAULT 0,
    -- 這次因 SPLIT / MERGE / RESHAPE 而終止的身分數。
    zone_ended INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 查詢一律是「某檔某段期間」或「全部某段期間」，兩者都吃得到。
CREATE INDEX IF NOT EXISTS idx_sr_identity_stats_symbol_created
    ON sr_identity_stats (symbol, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sr_identity_stats_analysis
    ON sr_identity_stats (analysis_id);

-- +goose Down
DROP TABLE IF EXISTS sr_identity_stats;
