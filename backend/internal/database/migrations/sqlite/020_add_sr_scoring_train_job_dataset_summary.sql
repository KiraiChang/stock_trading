-- +goose Up
-- 訓練資料診斷報告（見 docs/sr-zone-scoring.md「模型驗證與校準」）：
-- summarize_training_dataset() 產生的摘要（每個 symbol 的樣本數、role 分佈、
-- hold/break positive rate、特徵為 0 比例、RR reference 樣本數），原樣保存
-- 供人工判斷這次訓練出來的模型可不可信。NOT NULL DEFAULT '' 理由同
-- metrics 欄位（見 019 號 migration）：用 store.RawJSON 讀寫，不能是 SQL NULL。
ALTER TABLE sr_scoring_train_jobs ADD COLUMN dataset_summary TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE sr_scoring_train_jobs DROP COLUMN dataset_summary;
