-- 日 K 缺漏偵測的逐標的驗證簿記（issue.md I-091）。
-- 現況規格見 docs/database-schema.md 與 docs/architecture.md 的日 K 維護段。
--
-- 要解的問題：`evaluation_universe_sync` 的 `success` 只代表「請求沒失敗」，
-- 不代表「拿到了該有的資料」。2026-08-25 那輪 135 檔全成功，其中 `2867` 只回了 3 根
-- （視窗內有 7 個交易日），沒有任何東西報錯。偵測要獨立於回補流程，直接比對
-- 「實際日期集合」與「預期交易日集合」，而本表存的是**偵測自己的進度**。
--
-- **為什麼不是加欄位到 evaluation_universe**：
--   1. 那張表的 Upsert 是「重新匯入 selection report」的常態動作，把驗證簿記混進去
--      會讓研究決策與運維狀態互相干擾；
--   2. 候選來源日後可能不只池成員（watchlist 有同樣問題），綁在池表上會限制擴充；
--   3. 入退池是研究決策、驗證進度是運維狀態，兩者生命週期不同。
--
-- **刻意不建額外索引**：排序在 Go 端做（見下），查詢一律是
-- `WHERE timeframe = ? AND symbol IN (…)`，由 PRIMARY KEY 支撐。
-- 多一個沒人用的索引只是寫入成本。
--
-- **排序不靠 SQL**：候選的公平排序鍵是「沒有列者優先」，而「沒有列」不等於
-- 「欄位為 NULL」——首次出現的候選根本不會被 SELECT 回傳，不會變成排在最前面的 NULL
-- 而是直接消失。加上 `NULLS FIRST` 在 MySQL 不支援，因此定案由 Go 端合併排序，
-- repo 只負責把已有的 state 查回來（見 store/candle_verification_repo.go）。

-- +goose Up
CREATE TABLE IF NOT EXISTS candle_verification_state (
    symbol    VARCHAR(10) NOT NULL,
    timeframe VARCHAR(5)  NOT NULL,

    -- 可為 NULL：從未嘗試過。**首次出現的候選是「沒有列」而不是「NULL 欄位」**，
    -- 兩者在 SQL 上的行為不同，排序因此放在 Go 端。
    last_attempted_at TIMESTAMPTZ NULL,
    -- 可為 NULL：從未成功驗證過。
    -- **「確認有缺口」也算成功驗證**——那是一次有結論的驗證，只是結論是壞消息。
    last_verified_at  TIMESTAMPTZ NULL,

    -- 值域四個，**沒有 DEFAULT、寫入時必填**：給 DEFAULT '' 等於偷偷引入第五種狀態，
    -- 宣告只有四個值卻讓空字串合法。
    --
    -- verified    ：驗過，沒有缺口
    -- gap         ：驗過，確認有缺口（**仍是成功的驗證**）
    -- deferred    ：對照來源還沒發布到那一天，既不算缺口也不算失敗
    -- unavailable ：驗不了（請求失敗／格式變動／能力限制）
    --
    -- **deferred 必須在值域裡**：規格要求 deferred 也更新 last_attempted_at，
    -- 而 last_result 是 NOT NULL ＋ CHECK——首次就 deferred 的 symbol 沒有舊列可保留，
    -- 漏掉它會讓「正常發布延遲」變成寫入失敗。
    last_result VARCHAR(12) NOT NULL,

    -- **逐 symbol 的失敗計數，與來源層級的 circuit breaker 是兩回事**：
    -- 同一來源對五個不同 symbol 各失敗一次，這裡五筆都是 1，推導不出
    -- 「這個來源已連敗五次」。breaker 由實際送出且失敗的請求直接累計，存在行程內記憶體。
    consecutive_failures INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY (symbol, timeframe),
    CONSTRAINT ck_candle_verification_last_result
        CHECK (last_result IN ('verified', 'gap', 'deferred', 'unavailable'))
);

-- +goose Down
DROP TABLE IF EXISTS candle_verification_state;
