-- +goose Up
CREATE TABLE IF NOT EXISTS job_runs (
    id             BIGSERIAL    PRIMARY KEY,
    job_name       VARCHAR(20)  NOT NULL,
    status         VARCHAR(10)  NOT NULL DEFAULT 'running',
    symbols_total  INT          NOT NULL DEFAULT 0,
    symbols_failed INT          NOT NULL DEFAULT 0,
    error          TEXT,
    started_at     TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_job_runs_job_name ON job_runs (job_name);

-- +goose Down
DROP TABLE IF EXISTS job_runs;
