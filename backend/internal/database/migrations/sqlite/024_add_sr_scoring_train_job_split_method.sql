-- +goose Up
-- 保存訓練評估切分方式（time/random），供前端訓練紀錄與診斷顯示。
ALTER TABLE sr_scoring_train_jobs ADD COLUMN split_method TEXT;

-- +goose Down
ALTER TABLE sr_scoring_train_jobs DROP COLUMN split_method;
