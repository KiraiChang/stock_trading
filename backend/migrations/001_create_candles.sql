CREATE TABLE IF NOT EXISTS candles (
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol     VARCHAR(10)   NOT NULL,
    timeframe  VARCHAR(5)    NOT NULL,
    open       DECIMAL(10,2) NOT NULL,
    high       DECIMAL(10,2) NOT NULL,
    low        DECIMAL(10,2) NOT NULL,
    close      DECIMAL(10,2) NOT NULL,
    volume     BIGINT        NOT NULL,
    amount     DECIMAL(18,2) NOT NULL DEFAULT 0,
    ts         DATETIME(0)   NOT NULL,
    created_at DATETIME(0)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_symbol_tf_ts (symbol, timeframe, ts),
    INDEX idx_symbol_tf_ts (symbol, timeframe, ts DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
