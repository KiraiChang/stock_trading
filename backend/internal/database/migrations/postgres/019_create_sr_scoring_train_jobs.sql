-- +goose Up
-- 訓練任務可觀測化，見 sqlite 版本同名 migration 的說明。
CREATE TABLE sr_scoring_train_jobs (
    id            BIGSERIAL     PRIMARY KEY,
    job_id        VARCHAR(40)   NOT NULL UNIQUE,
    status        VARCHAR(15)   NOT NULL DEFAULT 'pending',
    symbols       TEXT          NOT NULL,
    timeframe     VARCHAR(5)    NOT NULL,
    fetch_limit   INT           NOT NULL,
    model_type    VARCHAR(30)   NOT NULL,
    rows          INT,
    sources       INT,
    -- metrics 用 store.RawJSON（純 string，非 sql.Null* 包裝）讀寫，NOT NULL
    -- DEFAULT '' 讓它在任務完成前永遠是空字串而不是 SQL NULL（RawJSON 掃
    -- NULL 會報錯）。
    metrics       TEXT          NOT NULL DEFAULT '',
    model_path    VARCHAR(255),
    model_version VARCHAR(20),
    error         TEXT,
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_sr_scoring_train_jobs_created ON sr_scoring_train_jobs (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS sr_scoring_train_jobs;
