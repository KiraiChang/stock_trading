-- +goose Up
CREATE TABLE IF NOT EXISTS indicator_snapshots (
    id          BIGSERIAL     PRIMARY KEY,
    symbol      VARCHAR(10)   NOT NULL,
    timeframe   VARCHAR(5)    NOT NULL,
    ts          TIMESTAMPTZ   NOT NULL,
    ma5         DECIMAL(10,4),
    ma10        DECIMAL(10,4),
    ma20        DECIMAL(10,4),
    ma60        DECIMAL(10,4),
    rsi14       DECIMAL(6,4),
    macd        DECIMAL(10,4),
    macd_signal DECIMAL(10,4),
    macd_hist   DECIMAL(10,4),
    bb_upper    DECIMAL(10,4),
    bb_middle   DECIMAL(10,4),
    bb_lower    DECIMAL(10,4),
    atr14       DECIMAL(10,4),
    vwap        DECIMAL(10,4),
    vol_ma20    BIGINT,
    vol_ratio   DECIMAL(6,4),
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (symbol, timeframe, ts)
);
CREATE INDEX IF NOT EXISTS idx_indicators_symbol_tf ON indicator_snapshots (symbol, timeframe, ts DESC);

-- +goose Down
DROP TABLE IF EXISTS indicator_snapshots;
