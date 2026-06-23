-- +goose Up
CREATE TABLE IF NOT EXISTS signals (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol      VARCHAR(10)   NOT NULL,
    signal_type VARCHAR(20)   NOT NULL,
    direction   VARCHAR(5)    NOT NULL,
    price       DECIMAL(10,2) NOT NULL,
    volume      BIGINT        NOT NULL,
    vol_ratio   DECIMAL(6,4),
    resistance  DECIMAL(10,2),
    support     DECIMAL(10,2),
    trend       VARCHAR(10),
    note        TEXT,
    ts          DATETIME(0)   NOT NULL,
    created_at  DATETIME(0)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_symbol_ts (symbol, ts DESC),
    INDEX idx_ts (ts DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS signals;
