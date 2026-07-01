-- +goose Up
CREATE TABLE IF NOT EXISTS stock_analysis_levels (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id  INTEGER NOT NULL,
    price        REAL    NOT NULL,
    type         TEXT    NOT NULL,
    strength     REAL    NOT NULL,
    method       TEXT    NOT NULL,
    status       TEXT    NOT NULL DEFAULT 'PENDING',
    broken_at    DATETIME,
    broken_price REAL,
    FOREIGN KEY(analysis_id) REFERENCES stock_analyses(id)
);

CREATE INDEX IF NOT EXISTS idx_stock_analysis_levels_analysis_id ON stock_analysis_levels(analysis_id);

-- +goose Down
DROP TABLE IF EXISTS stock_analysis_levels;
