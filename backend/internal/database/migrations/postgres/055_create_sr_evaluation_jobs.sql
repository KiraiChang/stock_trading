-- +goose Up
CREATE TABLE IF NOT EXISTS sr_evaluation_jobs (
    id BIGSERIAL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    symbols TEXT NOT NULL DEFAULT '[]',
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
    report JSONB NOT NULL DEFAULT 'null'::jsonb,
    error TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sr_evaluation_jobs_status ON sr_evaluation_jobs(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sr_evaluation_jobs_created ON sr_evaluation_jobs(created_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS sr_evaluation_jobs;
