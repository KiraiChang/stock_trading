-- +goose Up
CREATE TABLE IF NOT EXISTS chip_sync_jobs (
    id             BIGSERIAL    PRIMARY KEY,
    job_id         VARCHAR(40)  NOT NULL UNIQUE,
    mode           VARCHAR(20)  NOT NULL,
    symbols        TEXT         NOT NULL,
    data_types     TEXT         NOT NULL,
    from_date      DATE         NOT NULL,
    to_date        DATE         NOT NULL,
    force          BOOLEAN      NOT NULL DEFAULT FALSE,
    status         VARCHAR(15)  NOT NULL DEFAULT 'pending',
    symbols_total  INT          NOT NULL DEFAULT 0,
    symbols_done   INT          NOT NULL DEFAULT 0,
    symbols_failed INT          NOT NULL DEFAULT 0,
    failures       TEXT         NOT NULL DEFAULT '[]',
    error          TEXT,
    started_at     TIMESTAMPTZ,
    finished_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_chip_sync_jobs_created ON chip_sync_jobs (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS chip_sync_jobs;
