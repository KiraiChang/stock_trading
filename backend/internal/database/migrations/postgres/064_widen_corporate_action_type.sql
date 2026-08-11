-- corporate_actions.action_type 從 VARCHAR(16) 放寬到 VARCHAR(32)。
--
-- 起因：新增的 `CAPITAL_REDUCTION` 是 **17 字元**，超過原本的 16，寫入直接 22001。
-- 這與 migration 063（job_runs.job_name 裝不下 corporate_action_sync）是**完全相同的錯誤**——
-- 訂一個剛好夠用的長度，下一個名稱就撞上。
--
-- 兩支測試把關：`TestPostgresMigrationsActionTypeFitsAllConstants` 與
-- `TestPostgresMigrationsJobNameFitsLongestJob`，都是拿**程式碼裡真正的常數清單**去寫入，
-- 而不是寫死字串——日後新增更長的值會直接失敗。
--
-- sqlite 沒有對應檔案：該引擎的 action_type 是 TEXT，本來就沒有長度上限。

-- +goose Up
ALTER TABLE corporate_actions ALTER COLUMN action_type TYPE VARCHAR(32);

-- +goose Down
-- 回滾前必須先清掉超過 16 字元的資料，否則轉型會失敗。
DELETE FROM corporate_actions WHERE LENGTH(action_type) > 16;
ALTER TABLE corporate_actions ALTER COLUMN action_type TYPE VARCHAR(16);
