-- +goose Up
CREATE TABLE IF NOT EXISTS candles (
    id         BIGSERIAL     PRIMARY KEY,
    symbol     VARCHAR(10)   NOT NULL,
    timeframe  VARCHAR(5)    NOT NULL,
    open       DECIMAL(10,2) NOT NULL,
    high       DECIMAL(10,2) NOT NULL,
    low        DECIMAL(10,2) NOT NULL,
    close      DECIMAL(10,2) NOT NULL,
    volume     BIGINT        NOT NULL,
    amount     DECIMAL(18,2) NOT NULL DEFAULT 0,
    ts         TIMESTAMPTZ   NOT NULL,
    created_at TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (symbol, timeframe, ts)
);
CREATE INDEX IF NOT EXISTS idx_candles_symbol_tf_ts ON candles (symbol, timeframe, ts DESC);

-- +goose Down
DROP TABLE IF EXISTS candles;
