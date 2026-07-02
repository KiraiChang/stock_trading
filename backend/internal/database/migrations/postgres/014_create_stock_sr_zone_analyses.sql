-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_zone_analyses (
    id             BIGSERIAL     PRIMARY KEY,
    symbol         VARCHAR(10)   NOT NULL,
    timeframe      VARCHAR(5)    NOT NULL,
    analyzed_at    TIMESTAMPTZ   NOT NULL,
    current_price  DECIMAL(10,2) NOT NULL,
    model_version  VARCHAR(20)   NOT NULL,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_stock_sr_zone_analyses_symbol ON stock_sr_zone_analyses (symbol, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_zone_analyses;
