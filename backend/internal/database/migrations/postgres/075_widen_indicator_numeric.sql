-- 放寬 rsi14 與 vol_ratio 的數值精度。
-- 現況規格見 docs/database-schema.md 的 indicator_snapshots／signals，
-- RSI 的邊界語意見 docs/indicator-spec.md。
--
-- 要解的問題：兩個欄位原本是 DECIMAL(6,4)（上限 99.9999），但它們的合法值域都超過它——
--
--   * rsi14 的數學上界就是 100。CalcRSI 在 avgLoss == 0 時回傳 100，
--     2026-09-01 live 因為 2454 的 1m 收盤價連續 120 根完全相同而觸發，
--     整支標的的指標從 11:24 起就寫不進去（只留一行 warn）。
--   * vol_ratio = current_volume / MA20(volume)。MA20 走整數除法且有 ma > 0 守門，
--     所以分母最小是 1，於是 ratio <= current_volume；而 volume 是 BIGINT。
--     2026-09-01 live 實測 vol_ratio 最大已經到 92.5，離 99.9999 剩不到 8%。
--
-- 精度怎麼選：
--   * rsi14 -> DECIMAL(7,4)：RSI 值域是 [0, 100]，3 位整數位足夠且不多給。
--   * vol_ratio -> DECIMAL(23,4)：19 位整數位涵蓋非負 int64 的完整值域
--     （2^63 約 9.22e18），讓「不會溢位」在建構上成立，而不是依實測值估一個夠用的上限。
--     曾經考慮 DECIMAL(12,4)（約 1e8，比 live 1d 的最大單根量 4.2e9 還小，直接不夠）
--     與 DECIMAL(18,4)（約 1e14，只是營運容量選擇），兩者都證明不了不會溢位。
--
-- signals.vol_ratio 也是同一個型別、同一個來源（signal/breakout.go 把量比寫進訊號），
-- 所以兩張表要一起改；只改一張等於沒改。
--
-- 為什麼不加 cap 夾值：那會讓「100 倍爆量」與「1000 倍爆量」變成同一個數字，
-- 而那正是爆量訊號最需要分辨的資訊。
--
-- SET LOCAL lock_timeout：ALTER TABLE 要 ACCESS EXCLUSIVE，卡在盤中寫入時
-- 寧可失敗中止也不要排隊阻塞 indicator upsert 與 signal insert。
-- **必須寫在 migration 裡**——migration 是 backend 啟動時由 goose 用它自己的連線跑的
-- （database/migrate.go 的 goose.UpContext），在外部 psql session 下 SET 對它無效。
-- SET LOCAL 的作用域是 goose 包住這支 migration 的那個交易，結束即還原；
-- 因此本檔**不可**關掉交易（goose 那個 NO TRANSACTION 的 annotation），
-- 否則 SET LOCAL 會失去依附的交易。
--
-- ⚠️ **註解裡不要出現 goose 的 annotation 標記字串**（加號接 goose）。
-- goose 是掃「整行有沒有那個字串」，不是只看行首——寫在說明文字中間照樣會被當成
-- 真的 annotation，讓整支 migration 解析失敗（2026-09-01 實作時連踩兩次，
-- 錯誤訊息是 "failed to parse annotation line ... not supported: invalid annotation"）。

-- +goose Up
SET LOCAL lock_timeout = '5s';
ALTER TABLE indicator_snapshots ALTER COLUMN rsi14     TYPE DECIMAL(7,4);
ALTER TABLE indicator_snapshots ALTER COLUMN vol_ratio TYPE DECIMAL(23,4);
ALTER TABLE signals             ALTER COLUMN vol_ratio TYPE DECIMAL(23,4);

-- +goose Down
-- 縮回原精度。**任何超界的列都會讓它失敗**，這是刻意的：
-- indicator_snapshots 的列下一輪 Compute 只會補最新那個 ts（唯一鍵含 ts），
-- 中間的歷史列不會回來；signals 是事件紀錄，刪掉就永久消失。
-- 所以失敗時要先匯出留底並取得人工確認，不要直接刪。
-- 完整的回滾判準（含「純放寬的 migration 應該退 image 不退 schema」）見
-- docs/development-workflow.md「schema migration 上 live 的程序 › 回滾」。
SET LOCAL lock_timeout = '5s';
ALTER TABLE signals             ALTER COLUMN vol_ratio TYPE DECIMAL(6,4);
ALTER TABLE indicator_snapshots ALTER COLUMN vol_ratio TYPE DECIMAL(6,4);
ALTER TABLE indicator_snapshots ALTER COLUMN rsi14     TYPE DECIMAL(6,4);
