-- 擋掉價格非正的 K 棒（見 docs/database-schema.md 的 candles 章節）。postgres 版用 NOT VALID 容忍既有髒資料；
-- MySQL 沒有等價語法，ADD CONSTRAINT 會驗證既有列。目前 mysql 沒有實際資料
-- （dev / live 都是 postgres），所以直接加完整約束；日後若 mysql 真的存了髒資料，
-- 這支 migration 會失敗——那是正確行為，應該先清資料而不是放寬約束。

-- +goose Up
ALTER TABLE candles
    ADD CONSTRAINT ck_candles_positive_price
    CHECK (`open` > 0 AND high > 0 AND low > 0 AND `close` > 0);

-- +goose Down
ALTER TABLE candles DROP CHECK ck_candles_positive_price;
