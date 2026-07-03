-- +goose Up
ALTER TABLE sr_scoring_train_jobs ADD COLUMN split_method VARCHAR(20);

-- +goose Down
ALTER TABLE sr_scoring_train_jobs DROP COLUMN split_method;
