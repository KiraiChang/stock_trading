-- +goose Up
CREATE TABLE IF NOT EXISTS signals (
    id          BIGSERIAL     PRIMARY KEY,
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
    ts          TIMESTAMPTZ   NOT NULL,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_signals_symbol_ts ON signals (symbol, ts DESC);
CREATE INDEX IF NOT EXISTS idx_signals_ts ON signals (ts DESC);

-- +goose Down
DROP TABLE IF EXISTS signals;
