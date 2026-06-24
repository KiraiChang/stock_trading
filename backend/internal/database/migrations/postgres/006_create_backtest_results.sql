-- +goose Up
CREATE TABLE IF NOT EXISTS backtest_results (
    id            BIGSERIAL    PRIMARY KEY,
    job_id        VARCHAR(64)  NOT NULL UNIQUE REFERENCES backtest_jobs(job_id),
    strategy      VARCHAR(64)  NOT NULL,
    total_return  DECIMAL(10,4),
    annual_return DECIMAL(10,4),
    win_rate      DECIMAL(6,4),
    max_drawdown  DECIMAL(10,4),
    sharpe_ratio  DECIMAL(8,4),
    total_trades  INT,
    win_trades    INT,
    loss_trades   INT,
    avg_pnl       DECIMAL(10,4),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS backtest_results;
