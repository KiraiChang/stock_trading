-- +goose Up
CREATE TABLE IF NOT EXISTS backtest_trades (
    id          BIGSERIAL     PRIMARY KEY,
    job_id      VARCHAR(64)   NOT NULL REFERENCES backtest_jobs(job_id),
    symbol      VARCHAR(10)   NOT NULL,
    direction   VARCHAR(5)    NOT NULL,
    entry_time  TIMESTAMPTZ,
    exit_time   TIMESTAMPTZ,
    entry_price DECIMAL(10,2),
    exit_price  DECIMAL(10,2),
    size        DECIMAL(12,2),
    pnl         DECIMAL(12,2),
    pnl_pct     DECIMAL(8,4),
    commission  DECIMAL(10,2),
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_backtest_trades_job_id     ON backtest_trades (job_id);
CREATE INDEX IF NOT EXISTS idx_backtest_trades_job_symbol ON backtest_trades (job_id, symbol);

-- +goose Down
DROP TABLE IF EXISTS backtest_trades;
