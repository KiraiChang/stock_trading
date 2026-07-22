-- +goose Up
CREATE TABLE IF NOT EXISTS stock_symbols (
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol        VARCHAR(20)  NOT NULL,
    name          VARCHAR(120) NOT NULL,
    isin_code     VARCHAR(20)  NOT NULL DEFAULT '',
    market        VARCHAR(40)  NOT NULL DEFAULT '',
    security_type VARCHAR(40)  NOT NULL DEFAULT '',
    industry      VARCHAR(80)  NOT NULL DEFAULT '',
    cfi_code      VARCHAR(20)  NOT NULL DEFAULT '',
    remarks       VARCHAR(255) NOT NULL DEFAULT '',
    listed_date   DATE,
    is_listed     TINYINT(1)   NOT NULL DEFAULT 1,
    last_seen_at  DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at    DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_stock_symbols_symbol (symbol),
    INDEX idx_stock_symbols_is_listed (is_listed),
    INDEX idx_stock_symbols_security_type (security_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS stock_symbols;
