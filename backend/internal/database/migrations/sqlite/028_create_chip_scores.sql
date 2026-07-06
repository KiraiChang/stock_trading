-- +goose Up
CREATE TABLE IF NOT EXISTS chip_scores (
    id                   INTEGER  PRIMARY KEY AUTOINCREMENT,
    symbol               TEXT     NOT NULL,
    trade_date           DATE     NOT NULL,
    institutional_score  REAL     NOT NULL DEFAULT 0,
    margin_score         REAL     NOT NULL DEFAULT 0,
    broker_score         REAL     NOT NULL DEFAULT 0,
    concentration_score  REAL     NOT NULL DEFAULT 0,
    total_score          REAL     NOT NULL DEFAULT 0,
    signal               TEXT     NOT NULL DEFAULT 'NEUTRAL',
    reason               TEXT     NOT NULL DEFAULT '[]',
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol, trade_date)
);

CREATE INDEX IF NOT EXISTS idx_chip_scores_symbol_date
    ON chip_scores(symbol, trade_date DESC);

-- +goose Down
DROP TABLE IF EXISTS chip_scores;
