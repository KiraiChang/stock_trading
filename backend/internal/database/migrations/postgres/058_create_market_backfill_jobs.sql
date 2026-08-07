-- +goose Up
-- 股價（日K）回補任務追蹤。結構比照 chip_sync_jobs，但拿掉籌碼專屬的
-- data_types / from_date / to_date / force，改用單一 days（往前回補幾天）。
-- 目的與 chip_sync_jobs 相同：POST /market/backfill 原本是 fire-and-forget，
-- 20 檔要跑 4 分鐘卻沒有任何進度可查，中途 backend 重啟也不知道跑到哪。
-- failures 用 store.RawJSON 讀寫，NOT NULL DEFAULT 讓它在尚未有失敗紀錄前是 '[]'
-- 而不是 SQL NULL（RawJSON 是純 string、沒有實作 sql.Scanner）。
CREATE TABLE IF NOT EXISTS market_backfill_jobs (
    id             BIGSERIAL    PRIMARY KEY,
    job_id         VARCHAR(40)  NOT NULL UNIQUE,
    symbols        TEXT         NOT NULL,
    days           INT          NOT NULL,
    status         VARCHAR(15)  NOT NULL DEFAULT 'pending',
    symbols_total  INT          NOT NULL DEFAULT 0,
    symbols_done   INT          NOT NULL DEFAULT 0,
    symbols_failed INT          NOT NULL DEFAULT 0,
    failures       TEXT         NOT NULL DEFAULT '[]',
    error          TEXT,
    started_at     TIMESTAMPTZ,
    finished_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_market_backfill_jobs_created ON market_backfill_jobs (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS market_backfill_jobs;
