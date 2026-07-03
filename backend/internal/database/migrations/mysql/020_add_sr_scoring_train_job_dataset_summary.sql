-- +goose Up
-- 訓練資料診斷報告，見 sqlite 版本同名 migration 的說明。
ALTER TABLE sr_scoring_train_jobs ADD COLUMN dataset_summary TEXT NOT NULL DEFAULT ('');

-- +goose Down
ALTER TABLE sr_scoring_train_jobs DROP COLUMN dataset_summary;
