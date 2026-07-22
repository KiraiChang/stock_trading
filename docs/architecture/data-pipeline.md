# Data Pipeline

Data Pipeline 負責取得、清洗、時間對齊與持久化資料。它是所有分析、AI 與決策的上游，
但本身不做交易判斷、不訓練模型、不輸出買賣建議。

## 職責

- 連接外部資料源並處理 API 限制、錯誤、fallback 與重試策略。
- 將資料正規化後寫入 DB。
- 保留同步任務狀態，讓資料缺口可以追蹤。
- 對齊交易日期、K 棒時間、籌碼資料日期與部位資料時間。
- 提供下游穩定讀取契約。

## 現有模組歸位

| 類別 | 現有位置 | 說明 |
|------|----------|------|
| 市場資料來源 | `backend/internal/market` | FinMind、Fugle、Yahoo client 與資料源抽象 |
| 股票主檔同步 | `market.TWSEISINClient` / `market.StockSymbolSyncer` | 從 TWSE ISIN 清單同步有價證券主檔與 `is_listed` 狀態 |
| K 棒同步 | `market.Fetcher` | 歷史日 K、盤中資料、回補流程 |
| 排程資料工作 | `backend/internal/scheduler` | pre-market、intraday、daily-close、stock-symbol-sync 中的資料更新階段 |
| K 棒資料表 | `candles` | 下游指標、訊號、SR Zone、驗證使用 |
| 股票主檔資料表 | `stock_symbols` | symbol、名稱、ISIN、分類、產業與是否仍上市 |
| 籌碼 raw data | `institutional_trades` / `margin_trades` / `broker_trades` | 三大法人、融資融券、券商分點原始資料 |
| 籌碼同步任務 | `chip_sync_jobs` | 手動/回補籌碼任務進度 |
| 排程任務紀錄 | `job_runs` | daily/intraday 類任務狀態 |
| 監控清單 | `watchlist` | 決定排程與 UI 預設處理範圍 |
| 交易流水 | position transactions | immutable ledger，提供 Decision Pipeline 使用 |
| 部位 projection | `positions` | 由交易流水投影出的目前持股狀態 |

## 輸入

- FinMind 歷史日 K、分 K、三大法人、融資融券、券商分點。
- Fugle REST / WebSocket 行情。
- Yahoo 盤中行情。
- TWSE ISIN 上市（strMode=2）＋上櫃（strMode=4）有價證券清單，抓取行為見
  「[TWSE ISIN 同步策略](#twse-isin-同步策略)」。
- 使用者維護的 watchlist。
- 使用者新增的 BUY / SELL / ADJUSTMENT transaction。

## 輸出契約

- `candles`
- `stock_symbols`
- `institutional_trades`
- `margin_trades`
- `broker_trades`
- `watchlist`
- `positions`
- `position_transactions`
- `job_runs`
- `chip_sync_jobs`

## TWSE ISIN 同步策略

`stock_symbol_sync`（預設每日 06:30，`stock_symbols.cron`）依序抓上市（strMode=2）與
上櫃（strMode=4）兩個來源，合併去重後寫入 `stock_symbols`。

| 行為 | 現況 | 可調參數 |
|------|------|----------|
| 單次請求上限（含讀 body） | 300 秒 | `stock_symbols.timeout_sec` / `STOCK_SYMBOLS_TIMEOUT_SEC` |
| 來源之間的間隔 | 3 秒 | `stock_symbols.fetch_delay_sec` / `STOCK_SYMBOLS_FETCH_DELAY_SEC` |
| 整個 job 的上限 | 20 分鐘（`scheduler.stockSymbolSyncTimeout`） | 程式常數 |
| 失敗處理 | all-or-nothing，不自動重試 | — |

### 來源實測值（2026-07-22）

直接對 `strMode=2`（上市）發一次請求量到：**HTTP 200、7.5 MB、耗時 251 秒**。這解釋了
原本的失敗——30 秒與後來調到的 90 秒都遠低於實際下載時間，才會出現
`context deadline exceeded (... while reading body)`：連線與 header 都正常，是 body
讀到一半被 client timeout 中斷。timeout 因此設為 300 秒（量測值加約兩成餘裕），
job 上限同步放大到 20 分鐘（2 來源 × 300 秒 + 間隔 + 寫入）。

判斷是否需要再調整時，看 log 的 `elapsed` 欄位，不要憑感覺加秒數。

兩個秒數參數留 `0` 或不設定時，沿用 `internal/market/twse_isin.go` 的預設常數；預設值只
定義在程式裡一處，config 不重複寫死數字。request 會帶可辨識來源的 `User-Agent`
（`stock-trading/1.0 (personal market data sync)`）與 `Accept` / `Accept-Language`，
**刻意不偽裝成瀏覽器**——TWSE ISIN 是公開清單，抓取本身正當，站方要限流或聯絡時也才有依據。

### 為什麼不自動重試

抓取失敗（例如 2026-07-22 出現的 `context deadline exceeded ... while reading body`）時
整批放棄本次同步，**不做自動重試**，理由：

- 股票主檔異動頻率低（新上市／下市／改名），漏同步一天不影響既有標的的 K 線、指標與訊號，
  代價遠小於重試帶來的風險。
- all-or-nothing 是為了避免「只拿到半個市場」觸發大量誤下市；重試會拉長連續打同一站台的
  時間，反而提高被限流、拿到截斷回應的機率。
- 需要立即補資料時，人工觸發即可：前端的手動同步按鈕（`POST
  /api/v1/scheduler/stock-symbol-sync/run`），與 cron 走同一段邏輯、在背景執行。

診斷依據：每個來源成功或失敗都會記 `elapsed`（`twse isin source fetched` /
`twse isin source fetch failed`），要判斷 timeout 是否需要調整時看這個欄位；錯誤本身由
scheduler 記一次（`stock symbol sync failed`），不重複記錄。

## 不負責事項

- 不計算 `chip_scores`，那屬於 Analysis Pipeline。
- 不計算指標與訊號。
- 不建立 SR Zone。
- 不訓練或選擇模型。
- 不產生買賣建議。

## 近期整理方向

- 將文件中的「資料同步」與「分析計算」描述分開。
- 將 K 棒回補、籌碼回補、盤中資料同步標記為 Data Pipeline job。
- 後續若做程式重構，可先抽出 data job interface，避免 scheduler 直接混入分析與決策邏輯。
