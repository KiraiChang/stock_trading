-- +goose Up
CREATE TABLE IF NOT EXISTS stock_symbols (
    id            BIGSERIAL PRIMARY KEY,
    symbol        VARCHAR(20)  NOT NULL UNIQUE,
    name          VARCHAR(120) NOT NULL,
    isin_code     VARCHAR(20)  NOT NULL DEFAULT '',
    market        VARCHAR(40)  NOT NULL DEFAULT '',
    security_type VARCHAR(40)  NOT NULL DEFAULT '',
    industry      VARCHAR(80)  NOT NULL DEFAULT '',
    cfi_code      VARCHAR(20)  NOT NULL DEFAULT '',
    remarks       VARCHAR(255) NOT NULL DEFAULT '',
    listed_date   DATE,
    is_listed     BOOLEAN      NOT NULL DEFAULT TRUE,
    last_seen_at  TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_stock_symbols_is_listed ON stock_symbols(is_listed);
CREATE INDEX IF NOT EXISTS idx_stock_symbols_security_type ON stock_symbols(security_type);

-- +goose Down
DROP TABLE IF EXISTS stock_symbols;
