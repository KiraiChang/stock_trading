-- +goose Up
CREATE TABLE IF NOT EXISTS sr_evaluation_jobs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    symbols LONGTEXT NOT NULL,
    timeframe VARCHAR(10) NOT NULL DEFAULT '1d',
    fetch_limit BIGINT NOT NULL DEFAULT 1500,
    mode VARCHAR(30) NOT NULL DEFAULT 'evaluation',
    write_db BOOLEAN NOT NULL DEFAULT FALSE,
    replay_max_rows BIGINT NOT NULL DEFAULT 0,
    run_id VARCHAR(64),
    schema_version VARCHAR(64),
    pipeline_version VARCHAR(64),
    result_rows BIGINT,
    source_count BIGINT,
    report LONGTEXT NOT NULL,
    error TEXT,
    started_at DATETIME,
    finished_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_sr_evaluation_jobs_status (status, created_at),
    INDEX idx_sr_evaluation_jobs_created (created_at, id)
);

-- +goose Down
DROP TABLE IF EXISTS sr_evaluation_jobs;
