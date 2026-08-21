-- 事件鏈補上決策可見性旗標（todo.md T-041 的 review 發現 R1）。
-- 完整設計理由見 postgres 版的註解與 docs/database-schema.md。
--
-- mysql 差異：ADD COLUMN 沒有 IF NOT EXISTS（8.0 仍不支援），所以直接 ADD。
-- **本 engine 從未部署過**（見 docs/issue.md I-054），只由
-- scripts/test-mysql-migrations.sh 驗證 DDL。

-- +goose Up
ALTER TABLE event_instances ADD COLUMN decision_visible BOOLEAN NOT NULL DEFAULT TRUE;

-- 一次性回填，理由見 postgres 版：已終結的鏈不會再被寫入，不回填就會永遠標錯。
UPDATE event_instances SET decision_visible = FALSE
 WHERE event_family IN ('SUPPORT_RETEST', 'RESISTANCE_BREAKOUT');

-- +goose Down
ALTER TABLE event_instances DROP COLUMN decision_visible;
