-- job_runs 的「每個 job 取最新一筆」查詢索引。
-- 完整設計理由見 postgres 版的註解與 docs/api-reference.md 的 GET /scheduler/status。
--
-- mysql 差異：CREATE INDEX / DROP INDEX 沒有 IF NOT EXISTS / IF EXISTS，
-- 且 DROP INDEX 要帶 ON <table>。索引欄位的 DESC 在 8.0 才真正生效
-- （5.7 會接受語法但忽略），本專案的 mysql 目標版本是 8.0。
-- **本 engine 從未部署過**（見 docs/issue.md I-054），只由
-- scripts/test-mysql-migrations.sh 驗證 DDL。

-- +goose Up
CREATE INDEX idx_job_runs_job_name_started_at
    ON job_runs (job_name, started_at DESC);

-- +goose Down
DROP INDEX idx_job_runs_job_name_started_at ON job_runs;
