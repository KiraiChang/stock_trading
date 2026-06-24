-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER     PRIMARY KEY AUTOINCREMENT,
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    created_at    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS users;
