-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id INTEGER NOT NULL UNIQUE,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at DATETIME NOT NULL,
    market_bias VARCHAR(40) NOT NULL DEFAULT '',
    entry_permission_state VARCHAR(40) NOT NULL DEFAULT '',
    position_action VARCHAR(40) NOT NULL DEFAULT '',
    price_path_state VARCHAR(40) NOT NULL DEFAULT '',
    model_health_state VARCHAR(40) NOT NULL DEFAULT '',
    event_market_state VARCHAR(40) NOT NULL DEFAULT '',
    reason_codes TEXT NOT NULL DEFAULT '[]',
    decision_summary TEXT NOT NULL DEFAULT 'null',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(analysis_id) REFERENCES stock_sr_zone_analyses(id)
);
CREATE INDEX IF NOT EXISTS idx_stock_sr_decisions_symbol ON stock_sr_decisions(symbol, timeframe, analyzed_at DESC);

CREATE TABLE IF NOT EXISTS market_event_detections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id INTEGER NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at DATETIME NOT NULL,
    event_key VARCHAR(255) NOT NULL,
    event_type VARCHAR(80) NOT NULL,
    event_family VARCHAR(80) NOT NULL,
    event_scope VARCHAR(20) NOT NULL,
    zone_key VARCHAR(255) NOT NULL,
    direction VARCHAR(20) NOT NULL,
    state VARCHAR(40) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT 0,
    confidence REAL,
    price_level REAL,
    reason_codes TEXT NOT NULL DEFAULT '[]',
    event_json TEXT NOT NULL DEFAULT 'null',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(analysis_id) REFERENCES stock_sr_zone_analyses(id)
);
CREATE INDEX IF NOT EXISTS idx_market_event_detections_analysis ON market_event_detections(analysis_id);
CREATE INDEX IF NOT EXISTS idx_market_event_detections_symbol ON market_event_detections(symbol, timeframe, analyzed_at DESC);

CREATE TABLE IF NOT EXISTS market_event_states (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id INTEGER NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at DATETIME NOT NULL,
    event_key VARCHAR(255) NOT NULL,
    event_type VARCHAR(80) NOT NULL,
    event_family VARCHAR(80) NOT NULL,
    event_scope VARCHAR(20) NOT NULL,
    zone_key VARCHAR(255) NOT NULL,
    root_event_type VARCHAR(80) NOT NULL,
    latest_event_type VARCHAR(80) NOT NULL,
    direction VARCHAR(20) NOT NULL,
    state VARCHAR(40) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT 0,
    resolved_by VARCHAR(80),
    confidence REAL,
    price_level REAL,
    reason_codes TEXT NOT NULL DEFAULT '[]',
    state_json TEXT NOT NULL DEFAULT 'null',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(analysis_id) REFERENCES stock_sr_zone_analyses(id)
);
CREATE INDEX IF NOT EXISTS idx_market_event_states_analysis ON market_event_states(analysis_id);
CREATE INDEX IF NOT EXISTS idx_market_event_states_symbol_active ON market_event_states(symbol, timeframe, active, analyzed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS market_event_states;
DROP TABLE IF EXISTS market_event_detections;
DROP TABLE IF EXISTS stock_sr_decisions;
