-- 事件鏈補上決策可見性旗標（todo.md T-041 的 review 發現 R1）。
-- 完整設計理由見 postgres 版的註解與 docs/database-schema.md。
--
-- sqlite 差異：BOOLEAN 以 0/1 存；ALTER TABLE 不支援 IF NOT EXISTS / IF EXISTS，
-- 所以 Up/Down 都直接寫（DROP COLUMN 的用法比照 022 / 041 / 050 等既有 migration）。

-- +goose Up
ALTER TABLE event_instances ADD COLUMN decision_visible BOOLEAN NOT NULL DEFAULT 1;

-- 一次性回填，理由見 postgres 版：已終結的鏈不會再被寫入，不回填就會永遠標錯。
UPDATE event_instances SET decision_visible = 0
 WHERE event_family IN ('SUPPORT_RETEST', 'RESISTANCE_BREAKOUT');

-- +goose Down
ALTER TABLE event_instances DROP COLUMN decision_visible;
