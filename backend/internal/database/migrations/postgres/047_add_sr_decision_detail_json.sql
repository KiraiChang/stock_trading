-- +goose Up
ALTER TABLE stock_sr_decisions
    ADD COLUMN market_regime_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN data_quality_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN event_sequence_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN daily_price_action_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN price_path_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN daily_confirmation_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN defense_lines_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN rr_context_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN rr_gate_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN position_action_condition_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN market_context_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN confidence_explanation_json JSONB NOT NULL DEFAULT 'null'::jsonb,
    ADD COLUMN risk_notes_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN zone_summaries_json JSONB NOT NULL DEFAULT '{"nearest_decision_zone":null,"nearest_support_zone":null,"nearest_resistance_zone":null,"primary_structural_zone":null,"best_trade_zone":null,"primary_zone":null,"secondary_zones":[]}'::jsonb;

-- +goose Down
ALTER TABLE stock_sr_decisions
    DROP COLUMN zone_summaries_json,
    DROP COLUMN risk_notes_json,
    DROP COLUMN confidence_explanation_json,
    DROP COLUMN market_context_json,
    DROP COLUMN position_action_condition_json,
    DROP COLUMN rr_gate_json,
    DROP COLUMN rr_context_json,
    DROP COLUMN defense_lines_json,
    DROP COLUMN daily_confirmation_json,
    DROP COLUMN price_path_json,
    DROP COLUMN daily_price_action_json,
    DROP COLUMN event_sequence_json,
    DROP COLUMN data_quality_json,
    DROP COLUMN market_regime_json;
