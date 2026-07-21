-- +goose Up
-- 這批 detail 欄位在 SQLite/PostgreSQL 是 NOT NULL DEFAULT 'null'/'[]'/物件字串；MySQL 依
-- 本專案既有慣例（見 042/043/044），LONGTEXT 欄位一律 NOT NULL 但不帶 literal default
-- （避免相依 MySQL 8.0.13+ 的 TEXT expression default）。ADD 時先 NULL、回填後 MODIFY 成
-- NOT NULL；語意上與其他方言一致（皆不接受 NULL）。應用層（sr_zone_repo.Create）一律填值，
-- 若日後有寫入路徑遺漏欄位，MySQL 嚴格模式會直接報錯 fail-fast，而非默默存入非預期值。
ALTER TABLE stock_sr_decisions ADD COLUMN market_regime_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN data_quality_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN event_sequence_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN daily_price_action_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN price_path_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN daily_confirmation_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN defense_lines_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN rr_context_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN rr_gate_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN position_action_condition_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN market_context_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN confidence_explanation_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN risk_notes_json LONGTEXT NULL;
ALTER TABLE stock_sr_decisions ADD COLUMN zone_summaries_json LONGTEXT NULL;

UPDATE stock_sr_decisions SET market_regime_json = 'null' WHERE market_regime_json IS NULL;
UPDATE stock_sr_decisions SET data_quality_json = 'null' WHERE data_quality_json IS NULL;
UPDATE stock_sr_decisions SET event_sequence_json = '[]' WHERE event_sequence_json IS NULL;
UPDATE stock_sr_decisions SET daily_price_action_json = 'null' WHERE daily_price_action_json IS NULL;
UPDATE stock_sr_decisions SET price_path_json = 'null' WHERE price_path_json IS NULL;
UPDATE stock_sr_decisions SET daily_confirmation_json = 'null' WHERE daily_confirmation_json IS NULL;
UPDATE stock_sr_decisions SET defense_lines_json = 'null' WHERE defense_lines_json IS NULL;
UPDATE stock_sr_decisions SET rr_context_json = 'null' WHERE rr_context_json IS NULL;
UPDATE stock_sr_decisions SET rr_gate_json = 'null' WHERE rr_gate_json IS NULL;
UPDATE stock_sr_decisions SET position_action_condition_json = 'null' WHERE position_action_condition_json IS NULL;
UPDATE stock_sr_decisions SET market_context_json = '[]' WHERE market_context_json IS NULL;
UPDATE stock_sr_decisions SET confidence_explanation_json = 'null' WHERE confidence_explanation_json IS NULL;
UPDATE stock_sr_decisions SET risk_notes_json = '[]' WHERE risk_notes_json IS NULL;
UPDATE stock_sr_decisions SET zone_summaries_json = '{"nearest_decision_zone":null,"nearest_support_zone":null,"nearest_resistance_zone":null,"primary_structural_zone":null,"best_trade_zone":null,"primary_zone":null,"secondary_zones":[]}' WHERE zone_summaries_json IS NULL;

ALTER TABLE stock_sr_decisions MODIFY market_regime_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY data_quality_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY event_sequence_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY daily_price_action_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY price_path_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY daily_confirmation_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY defense_lines_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY rr_context_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY rr_gate_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY position_action_condition_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY market_context_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY confidence_explanation_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY risk_notes_json LONGTEXT NOT NULL;
ALTER TABLE stock_sr_decisions MODIFY zone_summaries_json LONGTEXT NOT NULL;

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
