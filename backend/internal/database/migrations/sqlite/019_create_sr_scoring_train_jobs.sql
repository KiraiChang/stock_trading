-- +goose Up
-- 訓練任務可觀測化（見 docs/sr-zone-scoring.md）：SR scoring 模型訓練目前是
-- 背景 goroutine 呼叫 Python 同步執行，只寫 log，前端無法查詢進度/結果。
-- 這張表讓 Go 在 Train() 建立任務時立即回傳 job_id，goroutine 依序更新
-- pending → running → done/failed，前端可輪詢查詢。
CREATE TABLE sr_scoring_train_jobs (
    id            INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id        TEXT     NOT NULL UNIQUE,
    status        TEXT     NOT NULL DEFAULT 'pending',
    symbols       TEXT     NOT NULL,
    timeframe     TEXT     NOT NULL,
    fetch_limit   INTEGER  NOT NULL,
    model_type    TEXT     NOT NULL,
    rows          INTEGER,
    sources       INTEGER,
    -- metrics 用 store.RawJSON（純 string，非 sql.Null* 包裝）讀寫，NOT NULL
    -- DEFAULT '' 讓它在任務完成前永遠是空字串而不是 SQL NULL——RawJSON 掃
    -- NULL 會直接報錯，且空字串在 JSON 輸出時本來就會序列化成 null，效果一樣。
    metrics       TEXT     NOT NULL DEFAULT '',
    model_path    TEXT,
    model_version TEXT,
    error         TEXT,
    started_at    DATETIME,
    finished_at   DATETIME,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_sr_scoring_train_jobs_created ON sr_scoring_train_jobs(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS sr_scoring_train_jobs;
