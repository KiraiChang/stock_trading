-- +goose Up
CREATE TABLE t087_b (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE t087_b;
