-- +goose Up
CREATE TABLE IF NOT EXISTS job_runs (
    id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    job_name       VARCHAR(20)  NOT NULL,
    status         VARCHAR(10)  NOT NULL DEFAULT 'running',
    symbols_total  INT          NOT NULL DEFAULT 0,
    symbols_failed INT          NOT NULL DEFAULT 0,
    error          TEXT,
    started_at     DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at    DATETIME(0),
    INDEX idx_job_runs_job_name (job_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS job_runs;
