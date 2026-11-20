-- +goose Up
-- 盤中逐輪、逐檔的存證（T-088＋I-103）。現況規格見 docs/database-schema.md 與 docs/yahoo-intraday-integration.md。
--
-- 要解的問題：盤中排程以前只有輪次層級的 job_runs 摘要（例如 fetch_failed:11），抓取失敗之後仍用舊 K 棒評估，
-- 「沒有新資料、用舊棒評估、沒觸發」跟「有新資料、評估後未觸發」分不開；逐檔寫入失敗更是完全看不到（I-103）。
--
-- **兩張表都以 job_runs 的 id（run_id）為鍵、⛔ 不設外鍵**：job_runs 的保留期清理會先刪那邊，
-- 這裡在同一步一起刪（開盤前），外鍵只會讓清理的順序變成隱性依賴。
--
-- intraday_rounds：一輪一列。排定的時點與實際執行區間分開記——盤中一輪與手動補抓共用一把鎖、依序執行，
-- 等鎖逾時的那一輪⛔ 不執行（outcome=skipped_lock_timeout）；在逐檔結果之前就失敗的輪次（例如讀不到自選清單）是 failed。
CREATE TABLE IF NOT EXISTS intraday_rounds (
    run_id            BIGINT      NOT NULL PRIMARY KEY,
    source            VARCHAR(10) NOT NULL,
    scheduled_at      TIMESTAMPTZ NOT NULL,
    -- 可為 NULL：等鎖逾時、沒有執行。
    lock_acquired_at  TIMESTAMPTZ NULL,
    -- 可為 NULL：還在跑、中斷，或沒有執行。
    exec_finished_at  TIMESTAMPTZ NULL,
    lock_wait_ms      BIGINT      NOT NULL DEFAULT 0,
    outcome           VARCHAR(24) NOT NULL,
    -- 逐檔存證寫失敗的次數（> 0 的輪次是「對帳不完整」）。
    evidence_failures INTEGER     NOT NULL DEFAULT 0,
    CONSTRAINT ck_intraday_rounds_source  CHECK (source IN ('scheduled', 'refill')),
    CONSTRAINT ck_intraday_rounds_outcome CHECK (outcome IN ('executed', 'skipped_lock_timeout', 'failed'))
);
CREATE INDEX IF NOT EXISTS idx_intraday_rounds_source_run ON intraday_rounds (source, run_id);
CREATE INDEX IF NOT EXISTS idx_intraday_rounds_scheduled_at ON intraday_rounds (scheduled_at);

-- intraday_symbol_runs：每一輪、每一檔一列。行情段在抓取與寫入之後 insert；評估段在那一檔評估完成之後 update
-- （手動補抓的列沒有評估段）。
CREATE TABLE IF NOT EXISTS intraday_symbol_runs (
    run_id         BIGINT      NOT NULL,
    symbol         VARCHAR(10) NOT NULL,
    source         VARCHAR(10) NOT NULL,
    data_at        TIMESTAMPTZ NOT NULL,
    data_status    VARCHAR(12) NOT NULL,
    -- 安全原因碼（封閉值域，⛔ 不含錯誤原文）；written／no_data 時是 NULL。
    data_cause     VARCHAR(32) NULL,
    -- 最後一根有效 K 棒的時間；written 以外是 NULL。
    last_bar_ts    TIMESTAMPTZ NULL,
    -- 寫入之後 DB 裡最近 N 根 1 分 K（N＝指標的 lookback）的 SHA-256；written 以外是 NULL。
    fingerprint    VARCHAR(64) NULL,
    -- 這一檔所在那一批的嘗試次數（1＝第一次就成功或失敗，重試最多 2 次）。
    fetch_attempts INTEGER     NOT NULL DEFAULT 1,
    -- 第一次失敗、後來 HTTP 成功（「HTTP 救回」⛔ 不等於「寫入成功」：救回的那一批裡仍可能有寫入失敗的檔）。
    http_recovered BOOLEAN     NOT NULL DEFAULT FALSE,
    eval_at        TIMESTAMPTZ NULL,
    eval_status    VARCHAR(20) NULL,
    eval_cause     VARCHAR(32) NULL,
    PRIMARY KEY (run_id, symbol),
    CONSTRAINT ck_intraday_symbol_runs_source CHECK (source IN ('scheduled', 'refill')),
    CONSTRAINT ck_intraday_symbol_runs_data_status
        CHECK (data_status IN ('written', 'no_data', 'store_failed', 'fetch_failed')),
    CONSTRAINT ck_intraday_symbol_runs_eval_status
        CHECK (eval_status IS NULL OR eval_status IN ('no_signal', 'signal_sent', 'signal_suppressed',
            'withheld_stale', 'degraded', 'evaluate_failed', 'not_evaluated'))
);
CREATE INDEX IF NOT EXISTS idx_intraday_symbol_runs_symbol ON intraday_symbol_runs (symbol, source, run_id);
CREATE INDEX IF NOT EXISTS idx_intraday_symbol_runs_data_at ON intraday_symbol_runs (data_at);

-- +goose Down
DROP TABLE IF EXISTS intraday_symbol_runs;
DROP TABLE IF EXISTS intraday_rounds;
