-- +goose Up
-- 籌碼資料 manual / backfill 同步任務追蹤（daily 模式沿用既有 job_runs 表，
-- 見 chip_daily_sync 這個 job_name）。此表保存結構化參數與批次失敗明細，
-- 供前端輪詢進度、失敗後從批次重跑。failures 用 store.RawJSON 讀寫，NOT NULL
-- DEFAULT 讓它在尚未有失敗紀錄前是 '[]' 而不是 SQL NULL。
CREATE TABLE IF NOT EXISTS chip_sync_jobs (
    id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    job_id         VARCHAR(40)  NOT NULL UNIQUE,
    mode           VARCHAR(20)  NOT NULL,
    symbols        TEXT         NOT NULL,
    data_types     TEXT         NOT NULL,
    from_date      DATE         NOT NULL,
    to_date        DATE         NOT NULL,
    force          TINYINT(1)   NOT NULL DEFAULT 0,
    status         VARCHAR(15)  NOT NULL DEFAULT 'pending',
    symbols_total  INT          NOT NULL DEFAULT 0,
    symbols_done   INT          NOT NULL DEFAULT 0,
    symbols_failed INT          NOT NULL DEFAULT 0,
    failures       TEXT         NOT NULL DEFAULT ('[]'),
    error          TEXT,
    started_at     DATETIME(0),
    finished_at    DATETIME(0),
    created_at     DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_chip_sync_jobs_created (created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS chip_sync_jobs;
