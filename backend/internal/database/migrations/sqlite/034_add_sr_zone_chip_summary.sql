-- +goose Up
-- chip_summary：整檔層級籌碼拆解 JSON（總分/訊號/四子分數/無資料旗標），
-- 由 Python score_symbol() 產生，供前端「共用籌碼面板」顯示。查無籌碼資料時
-- Python 會給 {"missing":true,...}，未帶此欄位的舊資料預設為 JSON null。
ALTER TABLE stock_sr_zone_analyses ADD COLUMN chip_summary TEXT NOT NULL DEFAULT 'null';

-- +goose Down
ALTER TABLE stock_sr_zone_analyses DROP COLUMN chip_summary;
