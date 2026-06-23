-- +goose Up
CREATE TABLE IF NOT EXISTS backtest_results (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id        TEXT    NOT NULL UNIQUE,
    strategy      TEXT    NOT NULL,
    total_return  REAL,
    annual_return REAL,
    win_rate      REAL,
    max_drawdown  REAL,
    sharpe_ratio  REAL,
    total_trades  INTEGER,
    win_trades    INTEGER,
    loss_trades   INTEGER,
    avg_pnl       REAL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(job_id) REFERENCES backtest_jobs(job_id)
);

-- +goose Down
DROP TABLE IF EXISTS backtest_results;
