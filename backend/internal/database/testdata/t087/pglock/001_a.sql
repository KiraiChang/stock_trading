-- +goose Up
CREATE TABLE t087_lock (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE t087_lock;
