-- +goose Up
-- 盤中逐輪、逐檔的存證（T-088＋I-103）。欄位說明見 postgres 版與 docs/database-schema.md。
CREATE TABLE IF NOT EXISTS intraday_rounds (
    run_id            BIGINT UNSIGNED NOT NULL PRIMARY KEY,
    source            VARCHAR(10) NOT NULL,
    scheduled_at      DATETIME(3) NOT NULL,
    lock_acquired_at  DATETIME(3) NULL,
    exec_finished_at  DATETIME(3) NULL,
    lock_wait_ms      BIGINT      NOT NULL DEFAULT 0,
    outcome           VARCHAR(24) NOT NULL,
    evidence_failures INT         NOT NULL DEFAULT 0,
    INDEX idx_intraday_rounds_source_run (source, run_id),
    INDEX idx_intraday_rounds_scheduled_at (scheduled_at),
    CONSTRAINT ck_intraday_rounds_source  CHECK (source IN ('scheduled', 'refill')),
    CONSTRAINT ck_intraday_rounds_outcome CHECK (outcome IN ('executed', 'skipped_lock_timeout', 'failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS intraday_symbol_runs (
    run_id         BIGINT UNSIGNED NOT NULL,
    symbol         VARCHAR(10) NOT NULL,
    source         VARCHAR(10) NOT NULL,
    data_at        DATETIME(3) NOT NULL,
    data_status    VARCHAR(12) NOT NULL,
    data_cause     VARCHAR(32) NULL,
    last_bar_ts    DATETIME(3) NULL,
    fingerprint    VARCHAR(64) NULL,
    fetch_attempts INT         NOT NULL DEFAULT 1,
    http_recovered BOOLEAN     NOT NULL DEFAULT FALSE,
    eval_at        DATETIME(3) NULL,
    eval_status    VARCHAR(20) NULL,
    eval_cause     VARCHAR(32) NULL,
    PRIMARY KEY (run_id, symbol),
    INDEX idx_intraday_symbol_runs_symbol (symbol, source, run_id),
    INDEX idx_intraday_symbol_runs_data_at (data_at),
    CONSTRAINT ck_intraday_symbol_runs_source CHECK (source IN ('scheduled', 'refill')),
    CONSTRAINT ck_intraday_symbol_runs_data_status
        CHECK (data_status IN ('written', 'no_data', 'store_failed', 'fetch_failed')),
    CONSTRAINT ck_intraday_symbol_runs_eval_status
        CHECK (eval_status IS NULL OR eval_status IN ('no_signal', 'signal_sent', 'signal_suppressed',
            'withheld_stale', 'degraded', 'evaluate_failed', 'not_evaluated'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS intraday_symbol_runs;
DROP TABLE IF EXISTS intraday_rounds;
