-- +goose Up
CREATE TABLE IF NOT EXISTS holdings (
    id          BIGSERIAL      PRIMARY KEY,
    symbol      VARCHAR(20)    NOT NULL,
    shares      NUMERIC(18,4)  NOT NULL,
    cost_price  NUMERIC(18,4)  NOT NULL,
    note        TEXT           NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_holdings_symbol ON holdings(symbol);

CREATE TABLE IF NOT EXISTS holding_analyses (
    id                   BIGSERIAL      PRIMARY KEY,
    holding_id           BIGINT         NOT NULL,
    symbol               VARCHAR(20)    NOT NULL,
    shares               NUMERIC(18,4)  NOT NULL,
    cost_price           NUMERIC(18,4)  NOT NULL,
    analyzed_at          TIMESTAMPTZ    NOT NULL,
    current_price        NUMERIC(18,4)  NOT NULL,
    sr_zone_analysis_id  BIGINT,
    action               VARCHAR(32)    NOT NULL,
    action_label         VARCHAR(64)    NOT NULL,
    stop_loss_price      NUMERIC(18,4),
    stop_loss_amount     NUMERIC(18,4),
    take_profit_price    NUMERIC(18,4),
    take_profit_amount   NUMERIC(18,4),
    add_on_trigger_price NUMERIC(18,4),
    add_on_amount        NUMERIC(18,4),
    unrealized_pnl       NUMERIC(18,4)  NOT NULL,
    unrealized_pnl_pct   NUMERIC(18,6)  NOT NULL,
    reason               TEXT           NOT NULL DEFAULT '[]',
    detail_json          TEXT           NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_holding_analyses_holding_id ON holding_analyses(holding_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_holding_analyses_symbol ON holding_analyses(symbol, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS holding_analyses;
DROP TABLE IF EXISTS holdings;
