-- +goose Up
CREATE TABLE t087_c (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE t087_c;
