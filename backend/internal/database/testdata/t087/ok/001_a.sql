-- +goose Up
CREATE TABLE t087_a (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE t087_a;
