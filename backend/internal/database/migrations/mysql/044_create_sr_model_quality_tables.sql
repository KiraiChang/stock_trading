-- +goose Up
CREATE TABLE IF NOT EXISTS stock_sr_model_metrics (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    train_job_id BIGINT UNSIGNED NOT NULL,
    job_id VARCHAR(64) NOT NULL UNIQUE,
    model_version VARCHAR(40) NOT NULL DEFAULT '',
    model_type VARCHAR(40) NOT NULL DEFAULT '',
    split_method VARCHAR(20) NOT NULL DEFAULT '',
    timeframe VARCHAR(10) NOT NULL DEFAULT '',
    -- rows 是 MySQL 保留字，需反引號（理由同 005 的 trigger）。
    `rows` BIGINT,
    sources BIGINT,
    hold_auc DOUBLE,
    hold_brier_score DOUBLE,
    hold_log_loss DOUBLE,
    hold_calibrated BOOLEAN,
    hold_test_rows BIGINT,
    break_auc DOUBLE,
    break_brier_score DOUBLE,
    break_log_loss DOUBLE,
    break_calibrated BOOLEAN,
    break_test_rows BIGINT,
    metrics_json LONGTEXT NOT NULL,
    dataset_summary_json LONGTEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (train_job_id) REFERENCES sr_scoring_train_jobs(id),
    INDEX idx_stock_sr_model_metrics_version (model_version, created_at)
);

CREATE TABLE IF NOT EXISTS stock_sr_model_governance (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    analysis_id BIGINT UNSIGNED NOT NULL UNIQUE,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    analyzed_at DATETIME NOT NULL,
    model_version VARCHAR(40) NOT NULL DEFAULT '',
    model_config_hash VARCHAR(40) NOT NULL DEFAULT '',
    health_state VARCHAR(40) NOT NULL DEFAULT '',
    average_edge_pp DOUBLE,
    directional_zone_count BIGINT,
    zone_count BIGINT,
    allow_entry BOOLEAN,
    max_entry_state VARCHAR(40) NOT NULL DEFAULT '',
    quality_flags LONGTEXT NOT NULL,
    warning_flags LONGTEXT NOT NULL,
    blocking_flags LONGTEXT NOT NULL,
    confidence_gate_json LONGTEXT NOT NULL,
    calibration_report_json LONGTEXT NOT NULL,
    walk_forward_report_json LONGTEXT NOT NULL,
    dataset_diagnostics_json LONGTEXT NOT NULL,
    governance_json LONGTEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (analysis_id) REFERENCES stock_sr_zone_analyses(id),
    INDEX idx_stock_sr_model_governance_symbol (symbol, timeframe, analyzed_at)
);

-- +goose Down
DROP TABLE IF EXISTS stock_sr_model_governance;
DROP TABLE IF EXISTS stock_sr_model_metrics;
