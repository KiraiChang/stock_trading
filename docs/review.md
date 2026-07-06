# 籌碼分析功能 Review

Review time: 2026-07-06 16:04:50 +08:00

Reviewed commits:

- `49c335e7a567e005b7708823e7f0f8a99f613755` - 加入籌碼資料拉取
- `554f392c5282152af65d8ea1a971c28cae761998` - 加入籌碼分析

## Findings

### High - 非交易日會寫入沿用舊資料的 chip_scores

`backend/internal/chip/sync.go:103` 以日曆日逐日迴圈，`backend/internal/chip/sync.go:108` 對每一天都計算並寫入 score。`computeAndStoreScore` 只用 `GetRange(..., date)` 取到該日前的法人/融資資料（`backend/internal/chip/sync.go:150`, `backend/internal/chip/sync.go:154`），`loadCandleContext` 註解也明確允許 date 當天沒有 candle 時退回最近一根 K 棒（`backend/internal/chip/sync.go:197`-`backend/internal/chip/sync.go:202`）。

這會在週末、國定假日、或當天資料尚未發布時，建立一筆 trade_date 為非交易日、但內容來自前一交易日的 `chip_scores`。後續 `GetLatest` 依 `trade_date DESC` 取最新（`backend/internal/store/chip_score_repo.go:129`-`backend/internal/store/chip_score_repo.go:135`），訊號加權也直接套最新籌碼分數（`backend/internal/signal/engine.go:119`-`backend/internal/signal/engine.go:128`），所以使用者可能看到週末日期的籌碼摘要，甚至用 stale score 加權技術訊號。

建議：計算 score 前確認該 date 有當日 candle 或至少有當日 institutional/margin 任一資料；沒有就 skip 該日，不寫 `chip_scores`。如果要允許降級，應把 score 的 trade_date 設成實際使用的 candle/trade date，而不是請求日期。

### High - 回測 chip filter 在缺籌碼資料時直接放行，會讓 filter 形同未啟用

`python/backtest/modular/backtester.py:122`-`python/backtest/modular/backtester.py:133` 中，只要 `chip_scores` 為 `None` 或訊號日沒有 score，就 `return True`。這表示使用者啟用 `use_chip_filter=true` 並設定 `chip_min_score=50` 時，只要資料庫沒有該日籌碼資料，所有進場訊號都會通過。

這和 `backend/internal/backtest/job.go` 的說明「未達門檻的進場訊號會被濾掉」不一致，也會讓回測結果過度樂觀，因為缺資料不是中性處理，而是完全不過濾。

建議：若 filter 已啟用，缺 score 應使用中性分數 `0` 與門檻比較，或在 job/result 中明確記錄缺資料比例並讓呼叫端選擇 fail-open/fail-closed。至少需要在回測結果中揭露「有多少筆訊號因缺籌碼資料而放行」。

### Medium - 前端用 UTC 日期，台灣時區早上會查詢/同步錯誤日期

`frontend/src/routes/Chips.svelte:39` 的 `todayStr()` 和 `frontend/src/routes/Chips.svelte:43` 的 `daysAgo()` 都用 `toISOString().substring(0, 10)`。`toISOString()` 是 UTC 日期；在台灣時間 00:00-07:59 期間，UTC 日期仍是前一天。這會影響 scores 查詢、broker 查詢與手動同步的 `to`（`frontend/src/routes/Chips.svelte:68`, `frontend/src/routes/Chips.svelte:76`, `frontend/src/routes/Chips.svelte:99`）。

結果是台灣使用者早上開頁面或按手動同步時，會查到昨天作為「今天」，並可能少同步一天資料。

建議：用本地日期格式化，或明確使用 Asia/Taipei 日期。例如以 `new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Taipei' })` 產生 `YYYY-MM-DD`。

### Medium - SR Zone 籌碼分量永遠取資料庫最新分數，和分析 K 線日期沒有對齊

`python/backtest/modular/sr_scoring/scoring.py:802` 直接呼叫 `fetch_latest_chip_score(symbol)`，沒有傳入 `before_date`。`python/db.py:79` 已支援 `before_date`，但目前沒有使用。當 K 線資料落後、離線重算舊資料、或資料庫中已有更晚的 chip score 時，SR Zone 的 trading score 會使用未來籌碼資料。

這會造成 lookahead bias，也會讓同一段歷史 K 線在不同時間重算得到不同的籌碼分量。

建議：用 `analyzed_at` 或最後一根 candle 的日期作為 `before_date` 傳入 `fetch_latest_chip_score(symbol, before_date=...)`。

## Notes

- 第二個 commit 已補上 `signal.NewEngine(..., chipScoreRepo, ...)` 的 wiring，因此整體看兩個 commit 合併後不會因 NewEngine 參數缺漏而編譯失敗。
- 券商分點目前是 FinMind unsupported stub，`broker_score` fallback 中性，這是明確設計；但前端文案仍寫「會抓取券商分點資料」，建議改成「若來源支援才同步」以免誤導使用者。
- 未執行測試；本次 review 以 diff 與必要上下文靜態檢查為主。
