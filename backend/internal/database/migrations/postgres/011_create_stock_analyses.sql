-- +goose Up
CREATE TABLE IF NOT EXISTS stock_analyses (
    id                      BIGSERIAL     PRIMARY KEY,
    symbol                  VARCHAR(10)   NOT NULL,
    timeframe               VARCHAR(5)    NOT NULL,
    analyzed_at             TIMESTAMPTZ   NOT NULL,
    current_price           DECIMAL(10,2) NOT NULL,
    trend                   VARCHAR(10)   NOT NULL,
    entry_status            VARCHAR(10)   NOT NULL,
    entry_direction         VARCHAR(5)    NOT NULL,
    entry_price             DECIMAL(10,2) NOT NULL,
    entry_reason            TEXT,
    stop_loss_atr           DECIMAL(10,2),
    stop_loss_structural    DECIMAL(10,2),
    stop_loss_composite     DECIMAL(10,2),
    take_profit_next_level  DECIMAL(10,2),
    take_profit_risk_reward DECIMAL(10,2),
    take_profit_atr         DECIMAL(10,2),
    trade_verification      TEXT,
    verified_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_stock_analyses_symbol ON stock_analyses (symbol, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_analyses;
