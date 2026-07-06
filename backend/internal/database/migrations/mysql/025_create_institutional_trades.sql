-- +goose Up
CREATE TABLE IF NOT EXISTS institutional_trades (
    id                        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol                    VARCHAR(20)  NOT NULL,
    trade_date                DATE         NOT NULL,
    foreign_net_buy           BIGINT       NOT NULL DEFAULT 0,
    investment_trust_net_buy  BIGINT       NOT NULL DEFAULT 0,
    dealer_net_buy            BIGINT       NOT NULL DEFAULT 0,
    total_net_buy             BIGINT       NOT NULL DEFAULT 0,
    created_at                DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at                DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_institutional_trades_symbol_date (symbol, trade_date),
    INDEX idx_institutional_trades_symbol_date (symbol, trade_date DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS institutional_trades;
