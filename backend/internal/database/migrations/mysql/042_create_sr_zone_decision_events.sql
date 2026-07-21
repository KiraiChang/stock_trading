-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_decisions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    analysis_id BIGINT UNSIGNED NOT NULL UNIQUE,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at DATETIME NOT NULL,
    market_bias VARCHAR(40) NOT NULL DEFAULT '',
    entry_permission_state VARCHAR(40) NOT NULL DEFAULT '',
    position_action VARCHAR(40) NOT NULL DEFAULT '',
    price_path_state VARCHAR(40) NOT NULL DEFAULT '',
    model_health_state VARCHAR(40) NOT NULL DEFAULT '',
    event_market_state VARCHAR(40) NOT NULL DEFAULT '',
    reason_codes LONGTEXT NOT NULL,
    decision_summary LONGTEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (analysis_id) REFERENCES stock_sr_zone_analyses(id),
    INDEX idx_stock_sr_decisions_symbol (symbol, timeframe, analyzed_at)
);

CREATE TABLE IF NOT EXISTS market_event_detections (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    analysis_id BIGINT UNSIGNED NOT NULL,
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
    active BOOLEAN NOT NULL DEFAULT FALSE,
    confidence DOUBLE,
    price_level DOUBLE,
    reason_codes LONGTEXT NOT NULL,
    event_json LONGTEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (analysis_id) REFERENCES stock_sr_zone_analyses(id),
    INDEX idx_market_event_detections_analysis (analysis_id),
    INDEX idx_market_event_detections_symbol (symbol, timeframe, analyzed_at)
);

CREATE TABLE IF NOT EXISTS market_event_states (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    analysis_id BIGINT UNSIGNED NOT NULL,
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
    active BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_by VARCHAR(80),
    confidence DOUBLE,
    price_level DOUBLE,
    reason_codes LONGTEXT NOT NULL,
    state_json LONGTEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (analysis_id) REFERENCES stock_sr_zone_analyses(id),
    INDEX idx_market_event_states_analysis (analysis_id),
    INDEX idx_market_event_states_symbol_active (symbol, timeframe, active, analyzed_at)
);

-- +goose Down
DROP TABLE IF EXISTS market_event_states;
DROP TABLE IF EXISTS market_event_detections;
DROP TABLE IF EXISTS stock_sr_decisions;
