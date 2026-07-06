-- +goose Up
CREATE TABLE IF NOT EXISTS margin_trades (
    id                 BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol             VARCHAR(20)    NOT NULL,
    trade_date         DATE           NOT NULL,
    margin_balance     BIGINT         NOT NULL DEFAULT 0,
    margin_change      BIGINT         NOT NULL DEFAULT 0,
    short_balance      BIGINT         NOT NULL DEFAULT 0,
    short_change       BIGINT         NOT NULL DEFAULT 0,
    margin_usage_rate  DECIMAL(10,4)  NULL,
    short_usage_rate   DECIMAL(10,4)  NULL,
    created_at         DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME(0)    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_margin_trades_symbol_date (symbol, trade_date),
    INDEX idx_margin_trades_symbol_date (symbol, trade_date DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS margin_trades;
