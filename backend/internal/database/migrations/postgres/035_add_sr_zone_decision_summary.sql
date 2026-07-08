-- +goose Up
-- decision_summary：T-019 決策摘要層 JSON（Market Regime、唯一 Action、Primary Zone、
-- Market Context、confidence 說明與 secondary zones），由 Python score_symbol() 產生。
-- 舊資料或尚未支援時預設為 JSON null。
ALTER TABLE stock_sr_zone_analyses ADD COLUMN decision_summary TEXT NOT NULL DEFAULT 'null';

-- +goose Down
ALTER TABLE stock_sr_zone_analyses DROP COLUMN decision_summary;