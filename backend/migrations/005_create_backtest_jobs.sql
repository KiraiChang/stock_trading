CREATE TABLE IF NOT EXISTS backtest_jobs (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    job_id      VARCHAR(64)  NOT NULL UNIQUE,
    type        VARCHAR(20)  NOT NULL DEFAULT 'backtest',
    strategy    VARCHAR(64)  NOT NULL,
    symbols     JSON         NOT NULL,
    timeframe   VARCHAR(5)   NOT NULL DEFAULT '1d',
    start_date  DATE         NOT NULL,
    end_date    DATE         NOT NULL,
    status      VARCHAR(10)  NOT NULL DEFAULT 'pending',
    trigger     VARCHAR(10)  NOT NULL DEFAULT 'manual',
    error       TEXT,
    created_at  DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at  DATETIME(0),
    finished_at DATETIME(0),
    INDEX idx_status   (status),
    INDEX idx_strategy (strategy)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
