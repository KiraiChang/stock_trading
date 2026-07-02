-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_zone_analyses (
    id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol         VARCHAR(10)   NOT NULL,
    timeframe      VARCHAR(5)    NOT NULL,
    analyzed_at    DATETIME(0)   NOT NULL,
    current_price  DECIMAL(10,2) NOT NULL,
    model_version  VARCHAR(20)   NOT NULL,
    created_at     DATETIME(0)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_symbol_created (symbol, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS stock_sr_zone_analyses;
