-- +goose Up
CREATE TABLE IF NOT EXISTS watchlists (
    id       SERIAL       PRIMARY KEY,
    symbol   VARCHAR(10)  NOT NULL UNIQUE,
    name     VARCHAR(100) NOT NULL,
    sector   VARCHAR(50),
    added_at TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS watchlists;
