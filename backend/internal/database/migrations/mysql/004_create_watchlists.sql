-- +goose Up
CREATE TABLE IF NOT EXISTS watchlists (
    id       INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol   VARCHAR(10)  NOT NULL,
    name     VARCHAR(100) NOT NULL,
    sector   VARCHAR(50),
    added_at DATETIME(0)  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_symbol (symbol)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS watchlists;
