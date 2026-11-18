-- +goose Up
-- 回測任務由哪一種 python 服務認領（worker／server；NULL＝還沒被認領，或上版前的舊資料）。
-- 認領是條件式的 UPDATE（只有 pending 才能變成 running），重啟時只收掉自己的遺留 running；
-- 狀態轉移與前提見 docs/database-schema.md 的 backtest_jobs（issue.md I-134）。
-- 可為 NULL、沒有預設值、不加 CHECK：值由程式控制、由測試鎖住。
ALTER TABLE backtest_jobs ADD COLUMN execution_owner VARCHAR(16);

-- +goose Down
ALTER TABLE backtest_jobs DROP COLUMN execution_owner;
