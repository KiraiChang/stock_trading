-- +goose Up
-- 測試用：要 ACCESS EXCLUSIVE 的變更（另一個連線持有表鎖時，要在 lock_timeout 內失敗）
ALTER TABLE t087_lock ADD COLUMN note TEXT;
-- +goose Down
ALTER TABLE t087_lock DROP COLUMN note;
