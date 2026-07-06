-- +goose Up
CREATE TABLE IF NOT EXISTS margin_trades (
    id                 INTEGER  PRIMARY KEY AUTOINCREMENT,
    symbol             TEXT     NOT NULL,
    trade_date         DATE     NOT NULL,
    margin_balance     INTEGER  NOT NULL DEFAULT 0,
    margin_change      INTEGER  NOT NULL DEFAULT 0,
    short_balance      INTEGER  NOT NULL DEFAULT 0,
    short_change       INTEGER  NOT NULL DEFAULT 0,
    margin_usage_rate  REAL,
    short_usage_rate   REAL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol, trade_date)
);

CREATE INDEX IF NOT EXISTS idx_margin_trades_symbol_date
    ON margin_trades(symbol, trade_date DESC);

-- +goose Down
DROP TABLE IF EXISTS margin_trades;
