-- +goose Up
-- 盤中逐輪、逐檔的存證（T-088＋I-103）。欄位說明見 postgres 版與 docs/database-schema.md。
CREATE TABLE IF NOT EXISTS intraday_rounds (
    run_id            INTEGER  NOT NULL PRIMARY KEY,
    source            TEXT     NOT NULL,
    scheduled_at      DATETIME NOT NULL,
    lock_acquired_at  DATETIME NULL,
    exec_finished_at  DATETIME NULL,
    lock_wait_ms      INTEGER  NOT NULL DEFAULT 0,
    outcome           TEXT     NOT NULL,
    evidence_failures INTEGER  NOT NULL DEFAULT 0,
    CONSTRAINT ck_intraday_rounds_source  CHECK (source IN ('scheduled', 'refill')),
    CONSTRAINT ck_intraday_rounds_outcome CHECK (outcome IN ('executed', 'skipped_lock_timeout', 'failed'))
);
CREATE INDEX IF NOT EXISTS idx_intraday_rounds_source_run ON intraday_rounds (source, run_id);
CREATE INDEX IF NOT EXISTS idx_intraday_rounds_scheduled_at ON intraday_rounds (scheduled_at);

CREATE TABLE IF NOT EXISTS intraday_symbol_runs (
    run_id         INTEGER  NOT NULL,
    symbol         TEXT     NOT NULL,
    source         TEXT     NOT NULL,
    data_at        DATETIME NOT NULL,
    data_status    TEXT     NOT NULL,
    data_cause     TEXT     NULL,
    last_bar_ts    DATETIME NULL,
    fingerprint    TEXT     NULL,
    fetch_attempts INTEGER  NOT NULL DEFAULT 1,
    http_recovered BOOLEAN  NOT NULL DEFAULT 0,
    eval_at        DATETIME NULL,
    eval_status    TEXT     NULL,
    eval_cause     TEXT     NULL,
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
DROP INDEX IF EXISTS idx_intraday_symbol_runs_data_at;
DROP INDEX IF EXISTS idx_intraday_symbol_runs_symbol;
DROP TABLE IF EXISTS intraday_symbol_runs;
DROP INDEX IF EXISTS idx_intraday_rounds_scheduled_at;
DROP INDEX IF EXISTS idx_intraday_rounds_source_run;
DROP TABLE IF EXISTS intraday_rounds;
