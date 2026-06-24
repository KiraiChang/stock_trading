-- +goose Up
CREATE TABLE IF NOT EXISTS backtest_jobs (
    id          BIGSERIAL    PRIMARY KEY,
    job_id      VARCHAR(64)  NOT NULL UNIQUE,
    type        VARCHAR(20)  NOT NULL DEFAULT 'backtest',
    strategy    VARCHAR(64)  NOT NULL,
    symbols     JSONB        NOT NULL,
    timeframe   VARCHAR(5)   NOT NULL DEFAULT '1d',
    start_date  DATE         NOT NULL,
    end_date    DATE         NOT NULL,
    status      VARCHAR(10)  NOT NULL DEFAULT 'pending',
    trigger     VARCHAR(10)  NOT NULL DEFAULT 'manual',
    error       TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_backtest_jobs_status   ON backtest_jobs (status);
CREATE INDEX IF NOT EXISTS idx_backtest_jobs_strategy ON backtest_jobs (strategy);

-- +goose Down
DROP TABLE IF EXISTS backtest_jobs;
