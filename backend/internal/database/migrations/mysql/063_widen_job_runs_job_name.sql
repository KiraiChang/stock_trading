-- job_runs.job_name 從 VARCHAR(20) 放寬到 VARCHAR(64)。
--
-- 起因（2026-08-11 正式環境）：新增的 `corporate_action_sync` 是 **21 字元**，
-- 超過原本的 20，寫入 job_runs 直接失敗：
--     value too long for type character varying(20) (SQLSTATE 22001)
--
-- 排程本身照跑（startRun 失敗只記 log 不中斷），所以症狀是「job 執行了但狀態頁看不到」——
-- 沒有人會因此發現，直到有人去翻 log。
--
-- 放寬而不是把 job 改名：20 這個上限沒有任何理由，下一個長名字還會再撞一次。
-- sqlite 沒有對應檔案：該引擎的 job_name 是 TEXT，本來就沒有長度上限。

-- +goose Up
ALTER TABLE job_runs MODIFY job_name VARCHAR(64) NOT NULL;

-- +goose Down
-- 回滾前必須先清掉超過 20 字元的資料，否則轉型會失敗。
DELETE FROM job_runs WHERE CHAR_LENGTH(job_name) > 20;
ALTER TABLE job_runs MODIFY job_name VARCHAR(20) NOT NULL;
