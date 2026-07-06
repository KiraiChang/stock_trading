-- +goose Up
-- 【籌碼分析整合】strength 讓訊號引擎依籌碼分數調整訊號強度（預設 1.0 代表
-- 未受籌碼調整的原始強度，加權後可能上修或下修，不是機率、不限制在 [0,1]）；
-- chip_signal 記錄套用調整時當下的籌碼訊號
-- （BULLISH/BEARISH/NEUTRAL/RISK），供前端/回測結構化查詢，不用再從 Note
-- 自由文字裡解析。
ALTER TABLE signals ADD COLUMN strength DECIMAL(5,4) NOT NULL DEFAULT 1.0;
ALTER TABLE signals ADD COLUMN chip_signal VARCHAR(20) NULL;

-- +goose Down
ALTER TABLE signals DROP COLUMN strength;
ALTER TABLE signals DROP COLUMN chip_signal;
