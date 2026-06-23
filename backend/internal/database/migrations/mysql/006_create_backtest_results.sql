-- +goose Up
CREATE TABLE IF NOT EXISTS backtest_results (
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    job_id        VARCHAR(64)  NOT NULL UNIQUE,
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
    created_at    DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (job_id) REFERENCES backtest_jobs(job_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS backtest_results;
