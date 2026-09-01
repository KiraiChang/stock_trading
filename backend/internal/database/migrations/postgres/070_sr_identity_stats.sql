-- 身分關聯決策的逐次分析統計（設計見 docs/database-schema.md 的 sr_identity_stats）。
-- 現況規格見 docs/database-schema.md 與 docs/sr-zone-scoring.md「可觀測性」。
--
-- 要解的問題：這些計數目前只以**單筆結構化 log** 輸出。log 答得出「這一次分析發生了
-- 什麼」，但答不出**趨勢**——「alias 命中率從 2% 爬到 30%」代表 zone 邊界漂移在惡化，
-- 而那要靠人每天 grep 才看得到。
--
-- **刻意不沿用 job_runs 的保留策略。** 立表當時（2026-08-21）job_runs 由 runPreMarket
-- 每天 08:50 `DeleteBefore(TodayTaipei())` **只留當天**——當時實測：前一晚 22:00 那輪排程
-- 的紀錄早上就查不到了。那正是本表要解的問題，複製那個策略等於白做。
--
-- **job_runs 的保留期已於 2026-08-25 改成 30 天**，上面那個前提不再成立；但本表仍然存在、
-- 也仍然不設保留期，因為兩者的粒度不同：job_runs 記每輪排程的成敗與標的數，本表記每次分析
-- 的身分關聯計數——job_runs 答不出「alias 命中率這個月往下掉了嗎」。
--
-- 量級：一次分析一列，所以每交易日約「watchlist 檔數 × 2」列（每日兩輪）。以 2026-08-31
-- production 的 11 檔估算是每交易日約 22 列 ≈ 5.5k~5.7k 列/年，欄位幾乎都是整數。
-- 檔數會變，要重估就套那條公式，不要沿用 22；也別拿它去乘 365——只有交易日會產出。
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
