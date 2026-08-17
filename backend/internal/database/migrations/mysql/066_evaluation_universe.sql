-- 評估標的池（T-040 Step 5 / Phase 2）。規格見
-- docs/evaluation-universe-selection-plan.md 的「Step 5 執行計畫書」。
--
-- 這張表**與 watchlists 分離**：watchlists 驅動六個流程，把 131 檔塞進去會讓每一個都
-- 乘上約 12 倍。本表只有一個用途——每日盤後更新這批標的的日 K。
-- **不參與任何交易決策或狀態推導。**
--
-- bucket_edge_low / bucket_edge_high 是刻意的反正規化，理由見 postgres 版的註解。
--
-- 欄位命名已避開 MySQL 保留字（見 docs/database-schema.md「避開 MySQL 保留字」）：
-- `source`、`active`、`note` 都不是保留字；`active` 在 MySQL 8.0 是非保留關鍵字，可直接使用。
-- mysql 從未在任何環境部署，唯一的驗證路徑是 scripts/test-mysql-migrations.sh（見 issue.md I-054）。

-- +goose Up
CREATE TABLE IF NOT EXISTS evaluation_universe (
    id               BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol           VARCHAR(10)    NOT NULL,
    bucket_hint      VARCHAR(32)    NOT NULL,
    bucket_edge_low  DECIMAL(18,10) NOT NULL,
    bucket_edge_high DECIMAL(18,10) NOT NULL,
    universe_version VARCHAR(32)    NOT NULL,
    universe_role    VARCHAR(16)    NOT NULL DEFAULT 'primary',
    selected_at      DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    source           VARCHAR(64)    NOT NULL,
    active           BOOLEAN        NOT NULL DEFAULT TRUE,
    -- TEXT 不能有 DEFAULT，所以這裡與 postgres/sqlite 不對稱：mysql 用 VARCHAR(1024)
    -- 才給得起 DEFAULT ''。省略該欄位的 INSERT 在三個 engine 上行為才一致
    -- （057 曾因為 mysql 沒有 DEFAULT 而不對稱，見 issue.md I-054 第 3 項）。
    note             VARCHAR(1024)  NOT NULL DEFAULT '',
    created_at       DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_evaluation_universe_symbol (symbol),
    INDEX idx_evaluation_universe_active (active, symbol),
    CONSTRAINT ck_evaluation_universe_edges
        CHECK (bucket_edge_low > 0 AND bucket_edge_high > bucket_edge_low)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS evaluation_universe;
