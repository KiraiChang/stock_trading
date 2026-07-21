-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_daily_candidates (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    analysis_id BIGINT UNSIGNED NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at DATETIME NOT NULL,
    price_low DOUBLE NOT NULL,
    price_high DOUBLE NOT NULL,
    label VARCHAR(80) NOT NULL DEFAULT '',
    role VARCHAR(20) NOT NULL DEFAULT '',
    source VARCHAR(40) NOT NULL DEFAULT '',
    lifecycle VARCHAR(40) NOT NULL DEFAULT '',
    decision_role VARCHAR(40) NOT NULL DEFAULT '',
    distance_pct DOUBLE,
    distance_label VARCHAR(40) NOT NULL DEFAULT '',
    reason TEXT NOT NULL,
    event_refs LONGTEXT NOT NULL,
    candidate_json LONGTEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (analysis_id) REFERENCES stock_sr_zone_analyses(id),
    INDEX idx_stock_sr_daily_candidates_analysis (analysis_id),
    INDEX idx_stock_sr_daily_candidates_symbol (symbol, timeframe, analyzed_at)
);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_daily_candidates;
