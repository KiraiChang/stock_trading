-- +goose Up
CREATE TABLE IF NOT EXISTS stock_analyses (
    id                      INTEGER  PRIMARY KEY AUTOINCREMENT,
    symbol                  TEXT     NOT NULL,
    timeframe               TEXT     NOT NULL,
    analyzed_at             DATETIME NOT NULL,
    current_price           REAL     NOT NULL,
    trend                   TEXT     NOT NULL,
    entry_status            TEXT     NOT NULL,
    entry_direction         TEXT     NOT NULL,
    entry_price             REAL     NOT NULL,
    entry_reason            TEXT,
    stop_loss_atr           REAL,
    stop_loss_structural    REAL,
    stop_loss_composite     REAL,
    take_profit_next_level  REAL,
    take_profit_risk_reward REAL,
    take_profit_atr         REAL,
    trade_verification      TEXT,
    verified_at             DATETIME,
    created_at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_stock_analyses_symbol ON stock_analyses(symbol, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_analyses;
