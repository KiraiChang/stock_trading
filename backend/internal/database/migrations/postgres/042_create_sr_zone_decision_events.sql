-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_decisions (
    id BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL UNIQUE REFERENCES stock_sr_zone_analyses(id),
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at TIMESTAMPTZ NOT NULL,
    market_bias VARCHAR(40) NOT NULL DEFAULT '',
    entry_permission_state VARCHAR(40) NOT NULL DEFAULT '',
    position_action VARCHAR(40) NOT NULL DEFAULT '',
    price_path_state VARCHAR(40) NOT NULL DEFAULT '',
    model_health_state VARCHAR(40) NOT NULL DEFAULT '',
    event_market_state VARCHAR(40) NOT NULL DEFAULT '',
    reason_codes TEXT NOT NULL DEFAULT '[]',
    decision_summary TEXT NOT NULL DEFAULT 'null',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_stock_sr_decisions_symbol ON stock_sr_decisions(symbol, timeframe, analyzed_at DESC);

CREATE TABLE IF NOT EXISTS market_event_detections (
    id BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL REFERENCES stock_sr_zone_analyses(id),
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at TIMESTAMPTZ NOT NULL,
    event_key VARCHAR(255) NOT NULL,
    event_type VARCHAR(80) NOT NULL,
    event_family VARCHAR(80) NOT NULL,
    event_scope VARCHAR(20) NOT NULL,
    zone_key VARCHAR(255) NOT NULL,
    direction VARCHAR(20) NOT NULL,
    state VARCHAR(40) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT FALSE,
    confidence DOUBLE PRECISION,
    price_level DOUBLE PRECISION,
    reason_codes TEXT NOT NULL DEFAULT '[]',
    event_json TEXT NOT NULL DEFAULT 'null',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_market_event_detections_analysis ON market_event_detections(analysis_id);
CREATE INDEX IF NOT EXISTS idx_market_event_detections_symbol ON market_event_detections(symbol, timeframe, analyzed_at DESC);

CREATE TABLE IF NOT EXISTS market_event_states (
    id BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL REFERENCES stock_sr_zone_analyses(id),
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at TIMESTAMPTZ NOT NULL,
    event_key VARCHAR(255) NOT NULL,
    event_type VARCHAR(80) NOT NULL,
    event_family VARCHAR(80) NOT NULL,
    event_scope VARCHAR(20) NOT NULL,
    zone_key VARCHAR(255) NOT NULL,
    root_event_type VARCHAR(80) NOT NULL,
    latest_event_type VARCHAR(80) NOT NULL,
    direction VARCHAR(20) NOT NULL,
    state VARCHAR(40) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_by VARCHAR(80),
    confidence DOUBLE PRECISION,
    price_level DOUBLE PRECISION,
    reason_codes TEXT NOT NULL DEFAULT '[]',
    state_json TEXT NOT NULL DEFAULT 'null',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_market_event_states_analysis ON market_event_states(analysis_id);
CREATE INDEX IF NOT EXISTS idx_market_event_states_symbol_active ON market_event_states(symbol, timeframe, active, analyzed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS market_event_states;
DROP TABLE IF EXISTS market_event_detections;
DROP TABLE IF EXISTS stock_sr_decisions;
