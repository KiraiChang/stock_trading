-- +goose Up
CREATE TABLE IF NOT EXISTS stock_analysis_levels (
    id           BIGSERIAL     PRIMARY KEY,
    analysis_id  BIGINT        NOT NULL REFERENCES stock_analyses(id),
    price        DECIMAL(10,2) NOT NULL,
    type         VARCHAR(10)   NOT NULL,
    strength     DECIMAL(6,4)  NOT NULL,
    method       VARCHAR(30)   NOT NULL,
    status       VARCHAR(15)   NOT NULL DEFAULT 'PENDING',
    broken_at    TIMESTAMPTZ,
    broken_price DECIMAL(10,2)
);

CREATE INDEX IF NOT EXISTS idx_stock_analysis_levels_analysis_id ON stock_analysis_levels (analysis_id);

-- +goose Down
DROP TABLE IF EXISTS stock_analysis_levels;
