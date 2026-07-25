-- +goose Up
-- 每個 (owner_type, owner_id) 至多一個 is_default portfolio；SQLite partial unique index 只約束 default 列。
CREATE UNIQUE INDEX IF NOT EXISTS uq_portfolios_default_owner
    ON portfolios(owner_type, owner_id) WHERE is_default = 1;

-- +goose Down
DROP INDEX IF EXISTS uq_portfolios_default_owner;
