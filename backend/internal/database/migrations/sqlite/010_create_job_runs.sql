-- +goose Up
CREATE TABLE IF NOT EXISTS job_runs (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_name       TEXT     NOT NULL,
    status         TEXT     NOT NULL DEFAULT 'running',
    symbols_total  INTEGER  NOT NULL DEFAULT 0,
    symbols_failed INTEGER  NOT NULL DEFAULT 0,
    error          TEXT,
    started_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_job_runs_job_name ON job_runs(job_name);

-- +goose Down
DROP TABLE IF EXISTS job_runs;
