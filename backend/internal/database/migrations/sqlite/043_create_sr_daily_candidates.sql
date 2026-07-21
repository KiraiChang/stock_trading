-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_daily_candidates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id INTEGER NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at DATETIME NOT NULL,
    price_low REAL NOT NULL,
    price_high REAL NOT NULL,
    label VARCHAR(80) NOT NULL DEFAULT '',
    role VARCHAR(20) NOT NULL DEFAULT '',
    source VARCHAR(40) NOT NULL DEFAULT '',
    lifecycle VARCHAR(40) NOT NULL DEFAULT '',
    decision_role VARCHAR(40) NOT NULL DEFAULT '',
    distance_pct REAL,
    distance_label VARCHAR(40) NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    event_refs TEXT NOT NULL DEFAULT '[]',
    candidate_json TEXT NOT NULL DEFAULT 'null',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(analysis_id) REFERENCES stock_sr_zone_analyses(id)
);
CREATE INDEX IF NOT EXISTS idx_stock_sr_daily_candidates_analysis ON stock_sr_daily_candidates(analysis_id);
CREATE INDEX IF NOT EXISTS idx_stock_sr_daily_candidates_symbol ON stock_sr_daily_candidates(symbol, timeframe, analyzed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_daily_candidates;
