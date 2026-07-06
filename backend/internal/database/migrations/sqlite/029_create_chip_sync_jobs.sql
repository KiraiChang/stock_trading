-- +goose Up
CREATE TABLE IF NOT EXISTS chip_sync_jobs (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id         TEXT     NOT NULL UNIQUE,
    mode           TEXT     NOT NULL,
    symbols        TEXT     NOT NULL,
    data_types     TEXT     NOT NULL,
    from_date      DATE     NOT NULL,
    to_date        DATE     NOT NULL,
    force          INTEGER  NOT NULL DEFAULT 0,
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

CREATE INDEX IF NOT EXISTS idx_chip_sync_jobs_created
    ON chip_sync_jobs(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS chip_sync_jobs;
