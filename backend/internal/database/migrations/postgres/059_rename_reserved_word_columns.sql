-- +goose Up
-- 把 4 個 MySQL 保留字欄位改名，讓 Go / Python 的查詢語句三個 engine 共用同一份 SQL。
--
-- 背景（docs/database-schema.md 的「欄位命名規範：避開 MySQL 保留字」）：
-- migration 的 DDL 可以用反引號迴避保留字，但那是
-- MySQL 專屬語法；repo 的查詢字串是三個 engine 共用的，加了反引號 postgres / sqlite 會
-- 直接語法錯誤。改名是唯一能讓「一份 SQL 到處跑」成立的做法。
--
-- **JSON / API 欄位名不變**（Go struct 的 json tag 保持 trigger / signal / force / rows），
-- 所以前端與 API 消費端完全不受影響。DB 欄位名與 JSON 欄位名刻意不同就是為了這個。
ALTER TABLE backtest_jobs           RENAME COLUMN trigger TO trigger_source;
ALTER TABLE chip_scores             RENAME COLUMN signal  TO signal_type;
ALTER TABLE chip_sync_jobs          RENAME COLUMN force   TO force_sync;
ALTER TABLE sr_scoring_train_jobs   RENAME COLUMN rows    TO row_count;
ALTER TABLE stock_sr_model_metrics  RENAME COLUMN rows    TO row_count;

-- +goose Down
ALTER TABLE stock_sr_model_metrics  RENAME COLUMN row_count    TO rows;
ALTER TABLE sr_scoring_train_jobs   RENAME COLUMN row_count    TO rows;
ALTER TABLE chip_sync_jobs          RENAME COLUMN force_sync   TO force;
ALTER TABLE chip_scores             RENAME COLUMN signal_type  TO signal;
ALTER TABLE backtest_jobs           RENAME COLUMN trigger_source TO trigger;
