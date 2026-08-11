-- corporate_actions.source 從 VARCHAR(32) 放寬到 VARCHAR(255)。
--
-- 起因：`TaiwanStockCapitalReductionReferencePrice` 是 **41 字元**，寫入 22001。
--
-- **這是同一個 session 內第三次撞上同一類錯誤**：
--   063  job_runs.job_name      VARCHAR(20) ← corporate_action_sync（21）
--   064  corporate_actions.action_type VARCHAR(16) ← CAPITAL_REDUCTION（17）
--   065  corporate_actions.source      VARCHAR(32) ← TaiwanStock…ReferencePrice（41）
--
-- 而 064 加的回歸測試**正好避開了這次**——它把 source 寫死成 'test' 只驗 action_type。
-- **逐欄測試、其餘欄位塞安全值，抓不到下一個欄位。**
-- 因此本次連同測試方式一起改：改用真實的 repo Upsert 寫入**每一組實際會出現的
-- (action_type, source) 組合**，所有欄位同時吃到正式值。
--
-- 寬度取 255 而不是「剛好夠用」：外部 dataset 名稱不受我們控制，訂緊只是在等下一次失敗。
-- sqlite 沒有對應檔案：該引擎是 TEXT，本來就沒有長度上限。

-- +goose Up
ALTER TABLE corporate_actions MODIFY source VARCHAR(255) NOT NULL;

-- +goose Down
DELETE FROM corporate_actions WHERE CHAR_LENGTH(source) > 32;
ALTER TABLE corporate_actions MODIFY source VARCHAR(32) NOT NULL;
