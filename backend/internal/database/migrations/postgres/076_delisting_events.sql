-- 終止上市名單的事件層與投影 provenance（現況見 docs/database-schema.md 與
-- docs/architecture.md 的下市過濾段；計畫書見 docs/todo.md T-071）。
--
-- **兩層設計的理由**：`delisting_events` 記錄的是**歷史事實**——「來源 S 說代號 X 在
-- 日期 D 終止上市」，同一代號可以有多代紀錄；`stock_symbols` 是 **current-state 主檔**，
-- 一個代號只有一列。把兩者混在一起正是誤判的成因：TWSE 的名單裡有 2002 年的
-- 「光寶電子(2301)」，而今天的 2301 是「光寶科」——直接寫主檔就會在光寶科將來下市時
-- 回填上一代證券的日期。分層之後，證不出身分的事件仍被保存供人工判讀。
--
-- **`stock_symbols` 的兩個新欄位是一組**：
--   delisted_date     終止上市日期
--   delisted_event_id 這個日期是誰寫的（provenance）
--
--   delisted_event_id 非 NULL → job 投影的，可被 delisting_reconcile 替換／清除
--   delisted_event_id 為 NULL 但 delisted_date 非 NULL → **人工填的，永不自動改動**
--
-- ⛔ **FK 是 RESTRICT，與鄰近的 job_run_id（SET NULL）相反，不可照抄**：
-- delisted_event_id 是**所有權標記**，被清成 NULL 會讓一筆 job-owned 的投影被誤認成
-- 人工值，從此不參與重算、**永久無法收斂**。事件本來就「永不 DELETE、消失只標記」
-- （missing_from_source_at），所以 RESTRICT 不會擋到任何正常路徑。
--
-- **CHECK 擋的是半套狀態**：delisted_date IS NULL + delisted_event_id 非 NULL
-- 代表「指向事件卻沒有日期」，那是撤銷或重新上市時只清一欄的實作錯誤訊號。

-- +goose Up
CREATE TABLE IF NOT EXISTS delisting_events (
    id                     BIGSERIAL    PRIMARY KEY,
    source                 VARCHAR(64)  NOT NULL,
    symbol                 VARCHAR(20)  NOT NULL,
    company_name           VARCHAR(120) NOT NULL,
    delisted_date          DATE         NOT NULL,
    first_seen_at          TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at           TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- 非 NULL = 該事件已從來源消失，**不參與投影**但保留供人工判讀。
    -- 只在 NULL → timestamp 的**轉換**當下寫入；持續缺席不得改寫它，
    -- 否則來源更正計數會每輪都 > 0，讓 job 永久 partial。
    missing_from_source_at TIMESTAMPTZ,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_delisting_events_identity UNIQUE (source, symbol, delisted_date)
);

CREATE INDEX IF NOT EXISTS idx_delisting_events_symbol ON delisting_events (symbol);

-- 每次**通過驗證並寫入**時記一列。縮水防護的基準取自這裡的上一筆 accepted。
--
-- ⛔ **「上一筆」一律以 `ORDER BY id DESC LIMIT 1` 取得**，不可只靠 fetched_at：
-- 兩次連續執行可能落在同一個時間戳，排序就變成未定義，基準會不確定。
-- job_run_id 只供追溯，不參與排序。
--
-- ⚠️ **job_run_id 的生命週期比快照短**：job_runs 只保留 30 天，而快照是縮水基準、
-- 必須長期保存。所以是 nullable + ON DELETE SET NULL——一般 FK 會讓 DeleteBefore
-- 失敗，CASCADE 更糟：會把縮水基準一起刪掉。
CREATE TABLE IF NOT EXISTS delisting_source_snapshots (
    id           BIGSERIAL    PRIMARY KEY,
    source       VARCHAR(64)  NOT NULL,
    job_run_id   BIGINT       REFERENCES job_runs(id) ON DELETE SET NULL,
    fetched_at   TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- 去重後的 canonical event 數，**不是** CSV 原始列數（否則去重會被誤判成縮水）。
    row_count    INTEGER      NOT NULL,
    content_hash VARCHAR(64)  NOT NULL,
    accepted     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_delisting_snapshots_source_id
    ON delisting_source_snapshots (source, id DESC);

ALTER TABLE stock_symbols ADD COLUMN IF NOT EXISTS delisted_date DATE;
ALTER TABLE stock_symbols ADD COLUMN IF NOT EXISTS delisted_event_id BIGINT;

ALTER TABLE stock_symbols
    ADD CONSTRAINT fk_stock_symbols_delisted_event
    FOREIGN KEY (delisted_event_id) REFERENCES delisting_events(id) ON DELETE RESTRICT;

ALTER TABLE stock_symbols
    ADD CONSTRAINT ck_stock_symbols_delisted_pair
    CHECK (delisted_event_id IS NULL OR delisted_date IS NOT NULL);

-- +goose Down
-- 順序與 Up 相反：先移除主檔的 FK／欄位，再刪事件表。
ALTER TABLE stock_symbols DROP CONSTRAINT IF EXISTS ck_stock_symbols_delisted_pair;
ALTER TABLE stock_symbols DROP CONSTRAINT IF EXISTS fk_stock_symbols_delisted_event;
ALTER TABLE stock_symbols DROP COLUMN IF EXISTS delisted_event_id;
ALTER TABLE stock_symbols DROP COLUMN IF EXISTS delisted_date;
DROP TABLE IF EXISTS delisting_source_snapshots;
DROP TABLE IF EXISTS delisting_events;
