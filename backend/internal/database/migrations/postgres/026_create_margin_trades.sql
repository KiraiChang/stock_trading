-- +goose Up
CREATE TABLE IF NOT EXISTS margin_trades (
    id                 BIGSERIAL      PRIMARY KEY,
    symbol             VARCHAR(20)    NOT NULL,
    trade_date         DATE           NOT NULL,
    margin_balance     BIGINT         NOT NULL DEFAULT 0,
    margin_change      BIGINT         NOT NULL DEFAULT 0,
    short_balance      BIGINT         NOT NULL DEFAULT 0,
    short_change       BIGINT         NOT NULL DEFAULT 0,
    margin_usage_rate  NUMERIC(10,4),
    short_usage_rate   NUMERIC(10,4),
    created_at         TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (symbol, trade_date)
);
CREATE INDEX IF NOT EXISTS idx_margin_trades_symbol_date ON margin_trades (symbol, trade_date DESC);

-- +goose Down
DROP TABLE IF EXISTS margin_trades;
