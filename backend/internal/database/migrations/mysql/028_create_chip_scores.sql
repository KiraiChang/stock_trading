-- +goose Up
-- reason 用 store.RawJSON（純 string，非 sql.Null* 包裝）讀寫，NOT NULL
-- DEFAULT 讓它在分數尚未計算理由前永遠是 '[]' 而不是 SQL NULL（RawJSON 掃
-- NULL 會報錯）。MySQL 8.0.13+ 才允許 TEXT 用括號包起來的 expression default。
CREATE TABLE IF NOT EXISTS chip_scores (
    id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol               VARCHAR(20)    NOT NULL,
    trade_date           DATE           NOT NULL,
    institutional_score  DECIMAL(10,4)  NOT NULL DEFAULT 0,
    margin_score         DECIMAL(10,4)  NOT NULL DEFAULT 0,
    broker_score         DECIMAL(10,4)  NOT NULL DEFAULT 0,
    concentration_score  DECIMAL(10,4)  NOT NULL DEFAULT 0,
    total_score          DECIMAL(10,4)  NOT NULL DEFAULT 0,
    signal               VARCHAR(20)    NOT NULL DEFAULT 'NEUTRAL',
    reason               TEXT           NOT NULL DEFAULT ('[]'),
    created_at           DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_chip_scores_symbol_date (symbol, trade_date),
    INDEX idx_chip_scores_symbol_date (symbol, trade_date DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS chip_scores;
