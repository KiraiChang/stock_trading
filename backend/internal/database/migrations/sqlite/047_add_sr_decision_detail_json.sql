-- +goose Up
ALTER TABLE stock_sr_decisions ADD COLUMN market_regime_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN data_quality_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN event_sequence_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE stock_sr_decisions ADD COLUMN daily_price_action_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN price_path_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN daily_confirmation_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN defense_lines_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN rr_context_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN rr_gate_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN position_action_condition_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN market_context_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE stock_sr_decisions ADD COLUMN confidence_explanation_json TEXT NOT NULL DEFAULT 'null';
ALTER TABLE stock_sr_decisions ADD COLUMN risk_notes_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE stock_sr_decisions ADD COLUMN zone_summaries_json TEXT NOT NULL DEFAULT '{"nearest_decision_zone":null,"nearest_support_zone":null,"nearest_resistance_zone":null,"primary_structural_zone":null,"best_trade_zone":null,"primary_zone":null,"secondary_zones":[]}';

-- +goose Down
ALTER TABLE stock_sr_decisions DROP COLUMN zone_summaries_json;
ALTER TABLE stock_sr_decisions DROP COLUMN risk_notes_json;
ALTER TABLE stock_sr_decisions DROP COLUMN confidence_explanation_json;
ALTER TABLE stock_sr_decisions DROP COLUMN market_context_json;
ALTER TABLE stock_sr_decisions DROP COLUMN position_action_condition_json;
ALTER TABLE stock_sr_decisions DROP COLUMN rr_gate_json;
ALTER TABLE stock_sr_decisions DROP COLUMN rr_context_json;
ALTER TABLE stock_sr_decisions DROP COLUMN defense_lines_json;
ALTER TABLE stock_sr_decisions DROP COLUMN daily_confirmation_json;
ALTER TABLE stock_sr_decisions DROP COLUMN price_path_json;
ALTER TABLE stock_sr_decisions DROP COLUMN daily_price_action_json;
ALTER TABLE stock_sr_decisions DROP COLUMN event_sequence_json;
ALTER TABLE stock_sr_decisions DROP COLUMN data_quality_json;
ALTER TABLE stock_sr_decisions DROP COLUMN market_regime_json;
