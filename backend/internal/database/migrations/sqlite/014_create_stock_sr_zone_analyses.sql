-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_zone_analyses (
    id             INTEGER  PRIMARY KEY AUTOINCREMENT,
    symbol         TEXT     NOT NULL,
    timeframe      TEXT     NOT NULL,
    analyzed_at    DATETIME NOT NULL,
    current_price  REAL     NOT NULL,
    model_version  TEXT     NOT NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_stock_sr_zone_analyses_symbol ON stock_sr_zone_analyses(symbol, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_zone_analyses;
