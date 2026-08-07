-- +goose Up
-- 見 postgres/059 的說明。MySQL 8.0+ 支援 RENAME COLUMN；
-- 舊名是保留字，在 ALTER 陳述句裡仍需反引號，新名則不需要——這正是本次改名的目的。
ALTER TABLE backtest_jobs           RENAME COLUMN `trigger` TO trigger_source;
ALTER TABLE chip_scores             RENAME COLUMN `signal`  TO signal_type;
ALTER TABLE chip_sync_jobs          RENAME COLUMN `force`   TO force_sync;
ALTER TABLE sr_scoring_train_jobs   RENAME COLUMN `rows`    TO row_count;
ALTER TABLE stock_sr_model_metrics  RENAME COLUMN `rows`    TO row_count;

-- +goose Down
ALTER TABLE stock_sr_model_metrics  RENAME COLUMN row_count      TO `rows`;
ALTER TABLE sr_scoring_train_jobs   RENAME COLUMN row_count      TO `rows`;
ALTER TABLE chip_sync_jobs          RENAME COLUMN force_sync     TO `force`;
ALTER TABLE chip_scores             RENAME COLUMN signal_type    TO `signal`;
ALTER TABLE backtest_jobs           RENAME COLUMN trigger_source TO `trigger`;
