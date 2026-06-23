CREATE TABLE IF NOT EXISTS backtest_trades (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id      TEXT     NOT NULL,
    symbol      TEXT     NOT NULL,
    direction   TEXT     NOT NULL,  -- BUY/SELL
    entry_time  DATETIME,
    exit_time   DATETIME,
    entry_price REAL,
    exit_price  REAL,
    size        REAL,
    pnl         REAL,
    pnl_pct     REAL,
    commission  REAL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(job_id) REFERENCES backtest_jobs(job_id)
);

CREATE INDEX IF NOT EXISTS idx_backtest_trades_job_id ON backtest_trades(job_id);
CREATE INDEX IF NOT EXISTS idx_backtest_trades_symbol ON backtest_trades(job_id, symbol);
