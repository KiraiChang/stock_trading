-- +goose Up
-- 模型設定可追溯性（見 docs/sr-zone-scoring.md「模型可追蹤性」/Python
-- model.py::compute_config_hash 說明）：model_version 只到「v1/v2」這種粗
-- 粒度，同一個版本底下換過幾次 DatasetConfig/zone builder 參數/
-- calibration_method 都無法從 model_version 分辨。model_config_hash 是這組
-- 訓練設定的短 hash，讓每筆分析都能被追溯到具體用哪組設定產生。
-- NOT NULL DEFAULT '' 理由同其他 RawJSON/文字欄位：用空字串表示「沒有這項
-- 資訊」（例如比這個欄位還舊的分析），不需要處理 SQL NULL。
ALTER TABLE stock_sr_zone_analyses ADD COLUMN model_config_hash TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE stock_sr_zone_analyses DROP COLUMN model_config_hash;
