-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_daily_candidates (
    id BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL REFERENCES stock_sr_zone_analyses(id),
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at TIMESTAMPTZ NOT NULL,
    price_low DOUBLE PRECISION NOT NULL,
    price_high DOUBLE PRECISION NOT NULL,
    label VARCHAR(80) NOT NULL DEFAULT '',
    role VARCHAR(20) NOT NULL DEFAULT '',
    source VARCHAR(40) NOT NULL DEFAULT '',
    lifecycle VARCHAR(40) NOT NULL DEFAULT '',
    decision_role VARCHAR(40) NOT NULL DEFAULT '',
    distance_pct DOUBLE PRECISION,
    distance_label VARCHAR(40) NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    event_refs TEXT NOT NULL DEFAULT '[]',
    candidate_json TEXT NOT NULL DEFAULT 'null',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_stock_sr_daily_candidates_analysis ON stock_sr_daily_candidates(analysis_id);
CREATE INDEX IF NOT EXISTS idx_stock_sr_daily_candidates_symbol ON stock_sr_daily_candidates(symbol, timeframe, analyzed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_daily_candidates;
