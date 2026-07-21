-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_model_metrics (
    id BIGSERIAL PRIMARY KEY,
    train_job_id BIGINT NOT NULL REFERENCES sr_scoring_train_jobs(id),
    job_id VARCHAR(64) NOT NULL UNIQUE,
    model_version VARCHAR(40) NOT NULL DEFAULT '',
    model_type VARCHAR(40) NOT NULL DEFAULT '',
    split_method VARCHAR(20) NOT NULL DEFAULT '',
    timeframe VARCHAR(10) NOT NULL DEFAULT '',
    rows BIGINT,
    sources BIGINT,
    hold_auc DOUBLE PRECISION,
    hold_brier_score DOUBLE PRECISION,
    hold_log_loss DOUBLE PRECISION,
    hold_calibrated BOOLEAN,
    hold_test_rows BIGINT,
    break_auc DOUBLE PRECISION,
    break_brier_score DOUBLE PRECISION,
    break_log_loss DOUBLE PRECISION,
    break_calibrated BOOLEAN,
    break_test_rows BIGINT,
    metrics_json TEXT NOT NULL DEFAULT 'null',
    dataset_summary_json TEXT NOT NULL DEFAULT 'null',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_stock_sr_model_metrics_version ON stock_sr_model_metrics(model_version, created_at DESC);

CREATE TABLE IF NOT EXISTS stock_sr_model_governance (
    id BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL UNIQUE REFERENCES stock_sr_zone_analyses(id),
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at TIMESTAMPTZ NOT NULL,
    model_version VARCHAR(40) NOT NULL DEFAULT '',
    model_config_hash VARCHAR(40) NOT NULL DEFAULT '',
    health_state VARCHAR(40) NOT NULL DEFAULT '',
    average_edge_pp DOUBLE PRECISION,
    directional_zone_count BIGINT,
    zone_count BIGINT,
    allow_entry BOOLEAN,
    max_entry_state VARCHAR(40) NOT NULL DEFAULT '',
    quality_flags TEXT NOT NULL DEFAULT '[]',
    warning_flags TEXT NOT NULL DEFAULT '[]',
    blocking_flags TEXT NOT NULL DEFAULT '[]',
    confidence_gate_json TEXT NOT NULL DEFAULT 'null',
    calibration_report_json TEXT NOT NULL DEFAULT 'null',
    walk_forward_report_json TEXT NOT NULL DEFAULT 'null',
    dataset_diagnostics_json TEXT NOT NULL DEFAULT 'null',
    governance_json TEXT NOT NULL DEFAULT 'null',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_stock_sr_model_governance_symbol ON stock_sr_model_governance(symbol, timeframe, analyzed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_model_governance;
DROP TABLE IF EXISTS stock_sr_model_metrics;
