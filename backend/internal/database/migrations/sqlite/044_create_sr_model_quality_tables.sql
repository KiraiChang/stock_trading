-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_model_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    train_job_id INTEGER NOT NULL,
    job_id VARCHAR(64) NOT NULL UNIQUE,
    model_version VARCHAR(40) NOT NULL DEFAULT '',
    model_type VARCHAR(40) NOT NULL DEFAULT '',
    split_method VARCHAR(20) NOT NULL DEFAULT '',
    timeframe VARCHAR(10) NOT NULL DEFAULT '',
    rows INTEGER,
    sources INTEGER,
    hold_auc REAL,
    hold_brier_score REAL,
    hold_log_loss REAL,
    hold_calibrated BOOLEAN,
    hold_test_rows INTEGER,
    break_auc REAL,
    break_brier_score REAL,
    break_log_loss REAL,
    break_calibrated BOOLEAN,
    break_test_rows INTEGER,
    metrics_json TEXT NOT NULL DEFAULT 'null',
    dataset_summary_json TEXT NOT NULL DEFAULT 'null',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(train_job_id) REFERENCES sr_scoring_train_jobs(id)
);
CREATE INDEX IF NOT EXISTS idx_stock_sr_model_metrics_version ON stock_sr_model_metrics(model_version, created_at DESC);

CREATE TABLE IF NOT EXISTS stock_sr_model_governance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id INTEGER NOT NULL UNIQUE,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at DATETIME NOT NULL,
    model_version VARCHAR(40) NOT NULL DEFAULT '',
    model_config_hash VARCHAR(40) NOT NULL DEFAULT '',
    health_state VARCHAR(40) NOT NULL DEFAULT '',
    average_edge_pp REAL,
    directional_zone_count INTEGER,
    zone_count INTEGER,
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
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(analysis_id) REFERENCES stock_sr_zone_analyses(id)
);
CREATE INDEX IF NOT EXISTS idx_stock_sr_model_governance_symbol ON stock_sr_model_governance(symbol, timeframe, analyzed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_model_governance;
DROP TABLE IF EXISTS stock_sr_model_metrics;
