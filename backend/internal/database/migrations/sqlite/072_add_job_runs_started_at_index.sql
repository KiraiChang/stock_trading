-- job_runs 的「每個 job 取最新一筆」查詢索引。
-- 完整設計理由見 postgres 版的註解與 docs/api-reference.md 的 GET /scheduler/status。
--
-- sqlite 差異：索引的 DESC 是支援的（3.8.3+），語法與 postgres 相同。

-- +goose Up
CREATE INDEX IF NOT EXISTS idx_job_runs_job_name_started_at
    ON job_runs (job_name, started_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_job_runs_job_name_started_at;
