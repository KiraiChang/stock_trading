-- +goose Up
CREATE TABLE IF NOT EXISTS broker_trades (
    id          BIGSERIAL     PRIMARY KEY,
    symbol      VARCHAR(20)   NOT NULL,
    trade_date  DATE          NOT NULL,
    broker_name VARCHAR(100)  NOT NULL,
    branch_name VARCHAR(100)  NOT NULL,
    buy_volume  BIGINT        NOT NULL DEFAULT 0,
    sell_volume BIGINT        NOT NULL DEFAULT 0,
    net_buy     BIGINT        NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (symbol, trade_date, broker_name, branch_name)
);

-- +goose Down
DROP TABLE IF EXISTS broker_trades;
