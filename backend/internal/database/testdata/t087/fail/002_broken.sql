-- +goose Up
-- 測試用：故意失敗的第二支（T-087 的逐支套用測試）
CREATE TABLE t087_b (id INTEGER PRIMARY KEY);
INSERT INTO t087_no_such_table VALUES (1);
-- +goose Down
DROP TABLE t087_b;
