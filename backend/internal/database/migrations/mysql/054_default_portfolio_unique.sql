-- +goose Up
-- MySQL 無 partial index：改用 functional key part，把非 default 列的 key 值設為 NULL
-- （unique index 視多個 NULL 互異），達到「每個 (owner_type, owner_id) 至多一筆 is_default」的約束。
-- 需 MySQL 8.0.13+（本專案定位 MySQL 8.0+）。
CREATE UNIQUE INDEX uq_portfolios_default_owner
    ON portfolios(owner_type, (IF(is_default, owner_id, NULL)));

-- +goose Down
DROP INDEX uq_portfolios_default_owner ON portfolios;
