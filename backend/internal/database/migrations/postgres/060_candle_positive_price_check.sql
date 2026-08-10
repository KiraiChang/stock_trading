-- 擋掉價格非正的 K 棒（見 docs/issue.md I-064）。
-- 寫入端已在 market/fetcher.go 的 toStoreCandles 擋一層；這裡是第二層，
-- 讓其他寫入路徑（手動 SQL、未來新增的匯入工具）也不可能繞過。

-- +goose Up
-- NOT VALID 是刻意的：live 目前仍有 4 列全零 K 棒尚未清理（清 live 資料屬另一個
-- 明確動作，需單獨授權）。NOT VALID 只約束**之後**的 INSERT/UPDATE，不回頭驗證既有列，
-- 所以這支 migration 在髒資料還在的情況下也能安全套用。
-- 清完那 4 列之後，再執行下面這行把約束升級成完整驗證：
--     ALTER TABLE candles VALIDATE CONSTRAINT ck_candles_positive_price;
ALTER TABLE candles
    ADD CONSTRAINT ck_candles_positive_price
    CHECK (open > 0 AND high > 0 AND low > 0 AND close > 0) NOT VALID;

-- +goose Down
ALTER TABLE candles DROP CONSTRAINT IF EXISTS ck_candles_positive_price;
