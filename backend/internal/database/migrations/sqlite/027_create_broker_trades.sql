-- +goose Up
CREATE TABLE IF NOT EXISTS broker_trades (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    symbol      TEXT     NOT NULL,
    trade_date  DATE     NOT NULL,
    broker_name TEXT     NOT NULL,
    branch_name TEXT     NOT NULL,
    buy_volume  INTEGER  NOT NULL DEFAULT 0,
    sell_volume INTEGER  NOT NULL DEFAULT 0,
    net_buy     INTEGER  NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol, trade_date, broker_name, branch_name)
);

-- +goose Down
DROP TABLE IF EXISTS broker_trades;
