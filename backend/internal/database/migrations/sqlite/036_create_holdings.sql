-- +goose Up
CREATE TABLE IF NOT EXISTS holdings (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    symbol      TEXT     NOT NULL,
    shares      REAL     NOT NULL,
    cost_price  REAL     NOT NULL,
    note        TEXT     NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_holdings_symbol ON holdings(symbol);

CREATE TABLE IF NOT EXISTS holding_analyses (
    id                   INTEGER  PRIMARY KEY AUTOINCREMENT,
    holding_id           INTEGER  NOT NULL,
    symbol               TEXT     NOT NULL,
    shares               REAL     NOT NULL,
    cost_price           REAL     NOT NULL,
    analyzed_at          DATETIME NOT NULL,
    current_price        REAL     NOT NULL,
    sr_zone_analysis_id  INTEGER,
    action               TEXT     NOT NULL,
    action_label         TEXT     NOT NULL,
    stop_loss_price      REAL,
    stop_loss_amount     REAL,
    take_profit_price    REAL,
    take_profit_amount   REAL,
    add_on_trigger_price REAL,
    add_on_amount        REAL,
    unrealized_pnl       REAL     NOT NULL,
    unrealized_pnl_pct   REAL     NOT NULL,
    reason               TEXT     NOT NULL DEFAULT '[]',
    detail_json          TEXT     NOT NULL DEFAULT '{}',
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_holding_analyses_holding_id ON holding_analyses(holding_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_holding_analyses_symbol ON holding_analyses(symbol, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS holding_analyses;
DROP TABLE IF EXISTS holdings;
