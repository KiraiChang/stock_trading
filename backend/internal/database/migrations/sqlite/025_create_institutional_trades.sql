-- +goose Up
CREATE TABLE IF NOT EXISTS institutional_trades (
    id                        INTEGER  PRIMARY KEY AUTOINCREMENT,
    symbol                    TEXT     NOT NULL,
    trade_date                DATE     NOT NULL,
    foreign_net_buy           INTEGER  NOT NULL DEFAULT 0,
    investment_trust_net_buy  INTEGER  NOT NULL DEFAULT 0,
    dealer_net_buy            INTEGER  NOT NULL DEFAULT 0,
    total_net_buy             INTEGER  NOT NULL DEFAULT 0,
    created_at                DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at                DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol, trade_date)
);

CREATE INDEX IF NOT EXISTS idx_institutional_trades_symbol_date
    ON institutional_trades(symbol, trade_date DESC);

-- +goose Down
DROP TABLE IF EXISTS institutional_trades;
