-- 放寬 rsi14 與 vol_ratio 的數值精度。完整設計理由見 postgres 版的註解。
--
-- mysql 差異：
--   * 改欄位型別用 MODIFY COLUMN，不是 ALTER COLUMN ... TYPE。
--   * MODIFY 會重寫整欄定義，所以原本的 NULL 特性要一起帶著
--     （這三欄在 002/003 都是可為 NULL、無 DEFAULT）。
--   * 沒有 SET LOCAL lock_timeout 的等價物；mysql 用 lock_wait_timeout，
--     但它是連線層設定而不是交易層，且 DDL 在 mysql 不具交易性——
--     **本 engine 從未部署過**（見 docs/issue.md I-054），只由
--     scripts/test-mysql-migrations.sh 驗證 DDL，所以這裡不另外設。
--     真要部署 mysql 時，鎖等待要在部署程序層面處理，不是在這支 migration 裡。
--   * DECIMAL 精度上限是 65，23 遠低於它。

-- +goose Up
ALTER TABLE indicator_snapshots MODIFY COLUMN rsi14     DECIMAL(7,4) NULL;
ALTER TABLE indicator_snapshots MODIFY COLUMN vol_ratio DECIMAL(23,4) NULL;
ALTER TABLE signals             MODIFY COLUMN vol_ratio DECIMAL(23,4) NULL;

-- +goose Down
-- 縮回原精度會讓任何超界的列失敗，這是刻意的。處置程序見
-- docs/development-workflow.md「schema migration 上 live 的程序 › 回滾」。
ALTER TABLE signals             MODIFY COLUMN vol_ratio DECIMAL(6,4) NULL;
ALTER TABLE indicator_snapshots MODIFY COLUMN vol_ratio DECIMAL(6,4) NULL;
ALTER TABLE indicator_snapshots MODIFY COLUMN rsi14     DECIMAL(6,4) NULL;
