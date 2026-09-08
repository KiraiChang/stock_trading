-- 終止上市名單的事件層與投影 provenance。設計理由見同編號的 postgres 檔，
-- 那裡有完整說明（兩層設計、provenance 語意、FK 為何是 RESTRICT）。
--
-- ⚠️ **本檔從未在任何環境部署過**（見 docs/issue.md I-054）：mysql 只靠
-- scripts/test-mysql-migrations.sh 驗 DDL，repo 層 CRUD 由
-- internal/database 的 TestMySQLMigrations… 測試涵蓋。
--
-- ⛔ **job_run_id 必須是 BIGINT UNSIGNED**：job_runs.id 是 BIGINT UNSIGNED
-- AUTO_INCREMENT（migration 010），型別不一致的話 FK 直接建不起來。

-- +goose Up
CREATE TABLE IF NOT EXISTS delisting_events (
    id                     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    source                 VARCHAR(64)  NOT NULL,
    symbol                 VARCHAR(20)  NOT NULL,
    company_name           VARCHAR(120) NOT NULL,
    delisted_date          DATE         NOT NULL,
    first_seen_at          DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at           DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- 非 NULL = 已從來源消失，不參與投影但保留供人工判讀。
    -- 只在 NULL → timestamp 的轉換當下寫入；持續缺席不得改寫。
    missing_from_source_at DATETIME(0)  NULL,
    created_at             DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_delisting_events_identity (source, symbol, delisted_date),
    INDEX idx_delisting_events_symbol (symbol)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS delisting_source_snapshots (
    id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    source       VARCHAR(64)     NOT NULL,
    job_run_id   BIGINT UNSIGNED NULL,
    fetched_at   DATETIME(0)     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- 去重後的 canonical event 數，不是 CSV 原始列數。
    row_count    INT             NOT NULL,
    content_hash VARCHAR(64)     NOT NULL,
    accepted     TINYINT(1)      NOT NULL DEFAULT 1,
    created_at   DATETIME(0)     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_delisting_snapshots_source_id (source, id DESC),
    CONSTRAINT fk_delisting_snapshots_job_run
        FOREIGN KEY (job_run_id) REFERENCES job_runs(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE stock_symbols ADD COLUMN delisted_date DATE NULL;
ALTER TABLE stock_symbols ADD COLUMN delisted_event_id BIGINT UNSIGNED NULL;

ALTER TABLE stock_symbols
    ADD CONSTRAINT fk_stock_symbols_delisted_event
    FOREIGN KEY (delisted_event_id) REFERENCES delisting_events(id) ON DELETE RESTRICT;

-- MySQL 8.0.16+ 才真正強制 CHECK；本專案的 compose 固定 8.4（docker-compose.mysql.yml）。
ALTER TABLE stock_symbols
    ADD CONSTRAINT ck_stock_symbols_delisted_pair
    CHECK (delisted_event_id IS NULL OR delisted_date IS NOT NULL);

-- +goose Down
ALTER TABLE stock_symbols DROP CHECK ck_stock_symbols_delisted_pair;
ALTER TABLE stock_symbols DROP FOREIGN KEY fk_stock_symbols_delisted_event;
ALTER TABLE stock_symbols DROP COLUMN delisted_event_id;
ALTER TABLE stock_symbols DROP COLUMN delisted_date;
DROP TABLE IF EXISTS delisting_source_snapshots;
DROP TABLE IF EXISTS delisting_events;
