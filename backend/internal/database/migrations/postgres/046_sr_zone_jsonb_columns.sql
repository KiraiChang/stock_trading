-- +goose Up
ALTER TABLE stock_sr_zone_analyses
    ALTER COLUMN evidence DROP DEFAULT,
    ALTER COLUMN evidence TYPE JSONB USING evidence::jsonb,
    ALTER COLUMN evidence SET DEFAULT 'null'::jsonb,
    ALTER COLUMN explanation DROP DEFAULT,
    ALTER COLUMN explanation TYPE JSONB USING explanation::jsonb,
    ALTER COLUMN explanation SET DEFAULT 'null'::jsonb,
    ALTER COLUMN scenario DROP DEFAULT,
    ALTER COLUMN scenario TYPE JSONB USING scenario::jsonb,
    ALTER COLUMN scenario SET DEFAULT 'null'::jsonb,
    ALTER COLUMN probability_context DROP DEFAULT,
    ALTER COLUMN probability_context TYPE JSONB USING probability_context::jsonb,
    ALTER COLUMN probability_context SET DEFAULT 'null'::jsonb,
    ALTER COLUMN period_summaries DROP DEFAULT,
    ALTER COLUMN period_summaries TYPE JSONB USING period_summaries::jsonb,
    ALTER COLUMN period_summaries SET DEFAULT '[]'::jsonb,
    ALTER COLUMN analysis_tips DROP DEFAULT,
    ALTER COLUMN analysis_tips TYPE JSONB USING analysis_tips::jsonb,
    ALTER COLUMN analysis_tips SET DEFAULT '[]'::jsonb,
    ALTER COLUMN chip_summary DROP DEFAULT,
    ALTER COLUMN chip_summary TYPE JSONB USING chip_summary::jsonb,
    ALTER COLUMN chip_summary SET DEFAULT 'null'::jsonb,
    ALTER COLUMN decision_summary DROP DEFAULT,
    ALTER COLUMN decision_summary TYPE JSONB USING decision_summary::jsonb,
    ALTER COLUMN decision_summary SET DEFAULT 'null'::jsonb;

ALTER TABLE stock_sr_zones
    ALTER COLUMN trading_score_breakdown TYPE JSONB USING trading_score_breakdown::jsonb,
    ALTER COLUMN features DROP DEFAULT,
    ALTER COLUMN features TYPE JSONB USING features::jsonb,
    ALTER COLUMN features SET DEFAULT 'null'::jsonb,
    ALTER COLUMN evidence DROP DEFAULT,
    ALTER COLUMN evidence TYPE JSONB USING evidence::jsonb,
    ALTER COLUMN evidence SET DEFAULT 'null'::jsonb,
    ALTER COLUMN explanation DROP DEFAULT,
    ALTER COLUMN explanation TYPE JSONB USING explanation::jsonb,
    ALTER COLUMN explanation SET DEFAULT 'null'::jsonb,
    ALTER COLUMN scenario DROP DEFAULT,
    ALTER COLUMN scenario TYPE JSONB USING scenario::jsonb,
    ALTER COLUMN scenario SET DEFAULT 'null'::jsonb,
    ALTER COLUMN probability_context DROP DEFAULT,
    ALTER COLUMN probability_context TYPE JSONB USING probability_context::jsonb,
    ALTER COLUMN probability_context SET DEFAULT 'null'::jsonb;

ALTER TABLE stock_sr_decisions
    ALTER COLUMN reason_codes DROP DEFAULT,
    ALTER COLUMN reason_codes TYPE JSONB USING reason_codes::jsonb,
    ALTER COLUMN reason_codes SET DEFAULT '[]'::jsonb,
    ALTER COLUMN decision_summary DROP DEFAULT,
    ALTER COLUMN decision_summary TYPE JSONB USING decision_summary::jsonb,
    ALTER COLUMN decision_summary SET DEFAULT 'null'::jsonb;

ALTER TABLE market_event_detections
    ALTER COLUMN reason_codes DROP DEFAULT,
    ALTER COLUMN reason_codes TYPE JSONB USING reason_codes::jsonb,
    ALTER COLUMN reason_codes SET DEFAULT '[]'::jsonb,
    ALTER COLUMN event_json DROP DEFAULT,
    ALTER COLUMN event_json TYPE JSONB USING event_json::jsonb,
    ALTER COLUMN event_json SET DEFAULT 'null'::jsonb;

ALTER TABLE market_event_states
    ALTER COLUMN reason_codes DROP DEFAULT,
    ALTER COLUMN reason_codes TYPE JSONB USING reason_codes::jsonb,
    ALTER COLUMN reason_codes SET DEFAULT '[]'::jsonb,
    ALTER COLUMN state_json DROP DEFAULT,
    ALTER COLUMN state_json TYPE JSONB USING state_json::jsonb,
    ALTER COLUMN state_json SET DEFAULT 'null'::jsonb;

ALTER TABLE stock_sr_daily_candidates
    ALTER COLUMN event_refs DROP DEFAULT,
    ALTER COLUMN event_refs TYPE JSONB USING event_refs::jsonb,
    ALTER COLUMN event_refs SET DEFAULT '[]'::jsonb,
    ALTER COLUMN candidate_json DROP DEFAULT,
    ALTER COLUMN candidate_json TYPE JSONB USING candidate_json::jsonb,
    ALTER COLUMN candidate_json SET DEFAULT 'null'::jsonb;

ALTER TABLE stock_sr_model_metrics
    ALTER COLUMN metrics_json DROP DEFAULT,
    ALTER COLUMN metrics_json TYPE JSONB USING metrics_json::jsonb,
    ALTER COLUMN metrics_json SET DEFAULT 'null'::jsonb,
    ALTER COLUMN dataset_summary_json DROP DEFAULT,
    ALTER COLUMN dataset_summary_json TYPE JSONB USING dataset_summary_json::jsonb,
    ALTER COLUMN dataset_summary_json SET DEFAULT 'null'::jsonb;

ALTER TABLE stock_sr_model_governance
    ALTER COLUMN quality_flags DROP DEFAULT,
    ALTER COLUMN quality_flags TYPE JSONB USING quality_flags::jsonb,
    ALTER COLUMN quality_flags SET DEFAULT '[]'::jsonb,
    ALTER COLUMN warning_flags DROP DEFAULT,
    ALTER COLUMN warning_flags TYPE JSONB USING warning_flags::jsonb,
    ALTER COLUMN warning_flags SET DEFAULT '[]'::jsonb,
    ALTER COLUMN blocking_flags DROP DEFAULT,
    ALTER COLUMN blocking_flags TYPE JSONB USING blocking_flags::jsonb,
    ALTER COLUMN blocking_flags SET DEFAULT '[]'::jsonb,
    ALTER COLUMN confidence_gate_json DROP DEFAULT,
    ALTER COLUMN confidence_gate_json TYPE JSONB USING confidence_gate_json::jsonb,
    ALTER COLUMN confidence_gate_json SET DEFAULT 'null'::jsonb,
    ALTER COLUMN calibration_report_json DROP DEFAULT,
    ALTER COLUMN calibration_report_json TYPE JSONB USING calibration_report_json::jsonb,
    ALTER COLUMN calibration_report_json SET DEFAULT 'null'::jsonb,
    ALTER COLUMN walk_forward_report_json DROP DEFAULT,
    ALTER COLUMN walk_forward_report_json TYPE JSONB USING walk_forward_report_json::jsonb,
    ALTER COLUMN walk_forward_report_json SET DEFAULT 'null'::jsonb,
    ALTER COLUMN dataset_diagnostics_json DROP DEFAULT,
    ALTER COLUMN dataset_diagnostics_json TYPE JSONB USING dataset_diagnostics_json::jsonb,
    ALTER COLUMN dataset_diagnostics_json SET DEFAULT 'null'::jsonb,
    ALTER COLUMN governance_json DROP DEFAULT,
    ALTER COLUMN governance_json TYPE JSONB USING governance_json::jsonb,
    ALTER COLUMN governance_json SET DEFAULT 'null'::jsonb;

ALTER TABLE stock_sr_regression_results
    ALTER COLUMN metrics_json DROP DEFAULT,
    ALTER COLUMN metrics_json TYPE JSONB USING metrics_json::jsonb,
    ALTER COLUMN metrics_json SET DEFAULT 'null'::jsonb;

-- +goose Down
ALTER TABLE stock_sr_regression_results
    ALTER COLUMN metrics_json DROP DEFAULT,
    ALTER COLUMN metrics_json TYPE TEXT USING metrics_json::text,
    ALTER COLUMN metrics_json SET DEFAULT 'null';

ALTER TABLE stock_sr_model_governance
    ALTER COLUMN quality_flags DROP DEFAULT,
    ALTER COLUMN quality_flags TYPE TEXT USING quality_flags::text,
    ALTER COLUMN quality_flags SET DEFAULT '[]',
    ALTER COLUMN warning_flags DROP DEFAULT,
    ALTER COLUMN warning_flags TYPE TEXT USING warning_flags::text,
    ALTER COLUMN warning_flags SET DEFAULT '[]',
    ALTER COLUMN blocking_flags DROP DEFAULT,
    ALTER COLUMN blocking_flags TYPE TEXT USING blocking_flags::text,
    ALTER COLUMN blocking_flags SET DEFAULT '[]',
    ALTER COLUMN confidence_gate_json DROP DEFAULT,
    ALTER COLUMN confidence_gate_json TYPE TEXT USING confidence_gate_json::text,
    ALTER COLUMN confidence_gate_json SET DEFAULT 'null',
    ALTER COLUMN calibration_report_json DROP DEFAULT,
    ALTER COLUMN calibration_report_json TYPE TEXT USING calibration_report_json::text,
    ALTER COLUMN calibration_report_json SET DEFAULT 'null',
    ALTER COLUMN walk_forward_report_json DROP DEFAULT,
    ALTER COLUMN walk_forward_report_json TYPE TEXT USING walk_forward_report_json::text,
    ALTER COLUMN walk_forward_report_json SET DEFAULT 'null',
    ALTER COLUMN dataset_diagnostics_json DROP DEFAULT,
    ALTER COLUMN dataset_diagnostics_json TYPE TEXT USING dataset_diagnostics_json::text,
    ALTER COLUMN dataset_diagnostics_json SET DEFAULT 'null',
    ALTER COLUMN governance_json DROP DEFAULT,
    ALTER COLUMN governance_json TYPE TEXT USING governance_json::text,
    ALTER COLUMN governance_json SET DEFAULT 'null';

ALTER TABLE stock_sr_model_metrics
    ALTER COLUMN metrics_json DROP DEFAULT,
    ALTER COLUMN metrics_json TYPE TEXT USING metrics_json::text,
    ALTER COLUMN metrics_json SET DEFAULT 'null',
    ALTER COLUMN dataset_summary_json DROP DEFAULT,
    ALTER COLUMN dataset_summary_json TYPE TEXT USING dataset_summary_json::text,
    ALTER COLUMN dataset_summary_json SET DEFAULT 'null';

ALTER TABLE stock_sr_daily_candidates
    ALTER COLUMN event_refs DROP DEFAULT,
    ALTER COLUMN event_refs TYPE TEXT USING event_refs::text,
    ALTER COLUMN event_refs SET DEFAULT '[]',
    ALTER COLUMN candidate_json DROP DEFAULT,
    ALTER COLUMN candidate_json TYPE TEXT USING candidate_json::text,
    ALTER COLUMN candidate_json SET DEFAULT 'null';

ALTER TABLE market_event_states
    ALTER COLUMN reason_codes DROP DEFAULT,
    ALTER COLUMN reason_codes TYPE TEXT USING reason_codes::text,
    ALTER COLUMN reason_codes SET DEFAULT '[]',
    ALTER COLUMN state_json DROP DEFAULT,
    ALTER COLUMN state_json TYPE TEXT USING state_json::text,
    ALTER COLUMN state_json SET DEFAULT 'null';

ALTER TABLE market_event_detections
    ALTER COLUMN reason_codes DROP DEFAULT,
    ALTER COLUMN reason_codes TYPE TEXT USING reason_codes::text,
    ALTER COLUMN reason_codes SET DEFAULT '[]',
    ALTER COLUMN event_json DROP DEFAULT,
    ALTER COLUMN event_json TYPE TEXT USING event_json::text,
    ALTER COLUMN event_json SET DEFAULT 'null';

ALTER TABLE stock_sr_decisions
    ALTER COLUMN reason_codes DROP DEFAULT,
    ALTER COLUMN reason_codes TYPE TEXT USING reason_codes::text,
    ALTER COLUMN reason_codes SET DEFAULT '[]',
    ALTER COLUMN decision_summary DROP DEFAULT,
    ALTER COLUMN decision_summary TYPE TEXT USING decision_summary::text,
    ALTER COLUMN decision_summary SET DEFAULT 'null';

ALTER TABLE stock_sr_zones
    ALTER COLUMN trading_score_breakdown TYPE TEXT USING trading_score_breakdown::text,
    ALTER COLUMN features DROP DEFAULT,
    ALTER COLUMN features TYPE TEXT USING features::text,
    ALTER COLUMN features SET DEFAULT 'null',
    ALTER COLUMN evidence DROP DEFAULT,
    ALTER COLUMN evidence TYPE TEXT USING evidence::text,
    ALTER COLUMN evidence SET DEFAULT 'null',
    ALTER COLUMN explanation DROP DEFAULT,
    ALTER COLUMN explanation TYPE TEXT USING explanation::text,
    ALTER COLUMN explanation SET DEFAULT 'null',
    ALTER COLUMN scenario DROP DEFAULT,
    ALTER COLUMN scenario TYPE TEXT USING scenario::text,
    ALTER COLUMN scenario SET DEFAULT 'null',
    ALTER COLUMN probability_context DROP DEFAULT,
    ALTER COLUMN probability_context TYPE TEXT USING probability_context::text,
    ALTER COLUMN probability_context SET DEFAULT 'null';

ALTER TABLE stock_sr_zone_analyses
    ALTER COLUMN evidence DROP DEFAULT,
    ALTER COLUMN evidence TYPE TEXT USING evidence::text,
    ALTER COLUMN evidence SET DEFAULT 'null',
    ALTER COLUMN explanation DROP DEFAULT,
    ALTER COLUMN explanation TYPE TEXT USING explanation::text,
    ALTER COLUMN explanation SET DEFAULT 'null',
    ALTER COLUMN scenario DROP DEFAULT,
    ALTER COLUMN scenario TYPE TEXT USING scenario::text,
    ALTER COLUMN scenario SET DEFAULT 'null',
    ALTER COLUMN probability_context DROP DEFAULT,
    ALTER COLUMN probability_context TYPE TEXT USING probability_context::text,
    ALTER COLUMN probability_context SET DEFAULT 'null',
    ALTER COLUMN period_summaries DROP DEFAULT,
    ALTER COLUMN period_summaries TYPE TEXT USING period_summaries::text,
    ALTER COLUMN period_summaries SET DEFAULT '[]',
    ALTER COLUMN analysis_tips DROP DEFAULT,
    ALTER COLUMN analysis_tips TYPE TEXT USING analysis_tips::text,
    ALTER COLUMN analysis_tips SET DEFAULT '[]',
    ALTER COLUMN chip_summary DROP DEFAULT,
    ALTER COLUMN chip_summary TYPE TEXT USING chip_summary::text,
    ALTER COLUMN chip_summary SET DEFAULT 'null',
    ALTER COLUMN decision_summary DROP DEFAULT,
    ALTER COLUMN decision_summary TYPE TEXT USING decision_summary::text,
    ALTER COLUMN decision_summary SET DEFAULT 'null';
