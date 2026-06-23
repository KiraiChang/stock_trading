CREATE TABLE IF NOT EXISTS backtest_jobs (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id      TEXT     NOT NULL UNIQUE,
    type        TEXT     NOT NULL DEFAULT 'backtest',
    strategy    TEXT     NOT NULL,
    symbols     TEXT     NOT NULL,  -- JSON array, e.g. ["2330","2454"]
    timeframe   TEXT     NOT NULL DEFAULT '1d',
    start_date  TEXT     NOT NULL,  -- YYYY-MM-DD
    end_date    TEXT     NOT NULL,
    status      TEXT     NOT NULL DEFAULT 'pending',  -- pending/running/done/failed
    trigger     TEXT     NOT NULL DEFAULT 'manual',   -- manual/scheduler
    error       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at  DATETIME,
    finished_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_backtest_jobs_status   ON backtest_jobs(status);
CREATE INDEX IF NOT EXISTS idx_backtest_jobs_strategy ON backtest_jobs(strategy);
