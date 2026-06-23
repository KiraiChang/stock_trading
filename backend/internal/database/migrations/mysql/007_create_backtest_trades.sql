-- +goose Up
CREATE TABLE IF NOT EXISTS backtest_trades (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    job_id      VARCHAR(64)   NOT NULL,
    symbol      VARCHAR(10)   NOT NULL,
    direction   VARCHAR(5)    NOT NULL,
    entry_time  DATETIME(0),
    exit_time   DATETIME(0),
    entry_price DECIMAL(10,2),
    exit_price  DECIMAL(10,2),
    size        DECIMAL(12,2),
    pnl         DECIMAL(12,2),
    pnl_pct     DECIMAL(8,4),
    commission  DECIMAL(10,2),
    created_at  DATETIME(0)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (job_id) REFERENCES backtest_jobs(job_id),
    INDEX idx_job_id (job_id),
    INDEX idx_job_symbol (job_id, symbol)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS backtest_trades;
