-- +goose Up
-- 每個 (owner_type, owner_id) 至多一個 is_default portfolio；partial unique index 只約束 default 列，
-- 非 default portfolio 不受限。取代原本只靠 application 端 racy NOT EXISTS 的做法。
CREATE UNIQUE INDEX IF NOT EXISTS uq_portfolios_default_owner
    ON portfolios(owner_type, owner_id) WHERE is_default;

-- +goose Down
DROP INDEX IF EXISTS uq_portfolios_default_owner;
