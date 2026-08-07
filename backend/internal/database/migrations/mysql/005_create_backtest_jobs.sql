-- +goose Up
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
    -- trigger 是 MySQL 保留字，DDL 必須用反引號括起來（postgres / sqlite 都容許裸寫，
    -- 所以這個錯誤一路到 2026-08-07 第一次真的在 MySQL 上跑 migration 才浮現）。
    `trigger`   VARCHAR(10)  NOT NULL DEFAULT 'manual',
    error       TEXT,
    created_at  DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at  DATETIME(0),
    finished_at DATETIME(0),
    INDEX idx_status   (status),
    INDEX idx_strategy (strategy)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS backtest_jobs;
