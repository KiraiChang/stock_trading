-- +goose Up
-- 見 postgres/058 的說明。欄位與型別比照本目錄的 029_create_chip_sync_jobs.sql。
CREATE TABLE IF NOT EXISTS market_backfill_jobs (
    id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    job_id         VARCHAR(40)  NOT NULL UNIQUE,
    symbols        TEXT         NOT NULL,
    days           INT          NOT NULL,
    status         VARCHAR(15)  NOT NULL DEFAULT 'pending',
    symbols_total  INT          NOT NULL DEFAULT 0,
    symbols_done   INT          NOT NULL DEFAULT 0,
    symbols_failed INT          NOT NULL DEFAULT 0,
    failures       TEXT         NOT NULL DEFAULT ('[]'),
    error          TEXT,
    started_at     DATETIME(0),
    finished_at    DATETIME(0),
    created_at     DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_market_backfill_jobs_created (created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS market_backfill_jobs;
