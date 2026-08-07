-- +goose Up
-- 見 postgres/058 的說明。欄位與型別比照本目錄的 029_create_chip_sync_jobs.sql。
CREATE TABLE IF NOT EXISTS market_backfill_jobs (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id         TEXT     NOT NULL UNIQUE,
    symbols        TEXT     NOT NULL,
    days           INTEGER  NOT NULL,
    status         TEXT     NOT NULL DEFAULT 'pending',
    symbols_total  INTEGER  NOT NULL DEFAULT 0,
    symbols_done   INTEGER  NOT NULL DEFAULT 0,
    symbols_failed INTEGER  NOT NULL DEFAULT 0,
    failures       TEXT     NOT NULL DEFAULT '[]',
    error          TEXT,
    started_at     DATETIME,
    finished_at    DATETIME,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_market_backfill_jobs_created
    ON market_backfill_jobs(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS market_backfill_jobs;
