-- +goose Up
-- 見 postgres/059 的說明。sqlite 的 RENAME COLUMN 需要 SQLite 3.25+，
-- modernc.org/sqlite 遠新於此；internal/store 的測試每次都實跑這份 migration。
ALTER TABLE backtest_jobs           RENAME COLUMN trigger TO trigger_source;
ALTER TABLE chip_scores             RENAME COLUMN signal  TO signal_type;
ALTER TABLE chip_sync_jobs          RENAME COLUMN force   TO force_sync;
ALTER TABLE sr_scoring_train_jobs   RENAME COLUMN rows    TO row_count;
ALTER TABLE stock_sr_model_metrics  RENAME COLUMN rows    TO row_count;

-- +goose Down
ALTER TABLE stock_sr_model_metrics  RENAME COLUMN row_count      TO rows;
ALTER TABLE sr_scoring_train_jobs   RENAME COLUMN row_count      TO rows;
ALTER TABLE chip_sync_jobs          RENAME COLUMN force_sync     TO force;
ALTER TABLE chip_scores             RENAME COLUMN signal_type    TO signal;
ALTER TABLE backtest_jobs           RENAME COLUMN trigger_source TO trigger;
