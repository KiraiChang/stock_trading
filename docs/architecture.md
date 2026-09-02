# 系統架構

## Pipeline 架構

系統升級後的文件邊界以四條 pipeline 描述；本檔保留整體架構總覽，細節放在
`docs/architecture/` 子目錄。

```
Data Pipeline → Analysis Pipeline → AI Pipeline → Decision Pipeline
```

| Pipeline | 子文件 | 職責摘要 |
|----------|--------|----------|
| Data Pipeline | [architecture/data-pipeline.md](./architecture/data-pipeline.md) | 外部資料取得、清洗、時間對齊、持久化 |
| Analysis Pipeline | [architecture/analysis-pipeline.md](./architecture/analysis-pipeline.md) | 指標、籌碼分數、SR Zone、分析快照與 evidence |
| AI Pipeline | [architecture/ai-pipeline.md](./architecture/ai-pipeline.md) | 模型訓練、模型狀態、推論、metrics |
| Decision Pipeline | [architecture/decision-pipeline.md](./architecture/decision-pipeline.md) | 交易行動、position sizing、停損停利、風控輸出 |

共同規則與索引見 [architecture/README.md](./architecture/README.md)。目前這是文件型架構拆分，
不代表程式碼已完全依四條 pipeline 重構。

## 整體資料流

```
FinMind API（歷史日K / 分K）      Fugle MarketData API（盤中即時，選填，見 fugle-integration.md）
    ↓ HTTP（每 5 分鐘 / 收盤後）        ↓ REST（quote/candles）+ WebSocket（candles channel）
Go Market Data Service（Fetcher）←──────┘
    ↓ BulkInsert
SQLite / MySQL / PostgreSQL（candles table）
    ↓ GetLatestN（120 根）
Indicator Engine（Go）
    ↓ Upsert
DB（indicator_snapshots）+ Redis（hash, TTL 5 min）
    ↓
Signal Engine（含 chip_scores 強度加權）
    ↓ Insert
DB（signals）
    ↓ WebSocket Broadcast
Frontend（Svelte，由 Go backend 直接 serve）
```

收盤後 `daily_close` 會依序執行日 K 更新、日線訊號掃描、SR Zone 驗證與籌碼日結同步。
籌碼同步由 `chip.Syncer` 寫入 institutional/margin/broker raw tables 與 `chip_scores`，
失敗時只影響自己的 `job_runs` 紀錄，不會回滾 K 線或訊號結果。

Fugle 目前預設關閉（`fugle.enabled: false`），已完成 REST/WebSocket client 與
`cmd/fugle-check` 驗證工具，尚未接上 `Fetcher`/`scheduler` 的自動排程；詳見
[fugle-integration.md](./fugle-integration.md)。

FinMind 盤中分K排程也預設關閉（`finmind.intraday_enabled: false`）——該
dataset（`TaiwanStockKBar`）需要 Sponsor 級以上 token，帳號等級不足時
`runIntradayJob` 會直接跳過，不會每 5 分鐘對 FinMind 發出注定失敗的請求；
升級帳號後改成 `true` 即可恢復，見 [finmind-integration.md](./finmind-integration.md)。

---

## 模組關係

```
cmd/server/main.go
    ├── store（DB + Redis repos，含 UserRepo）
    ├── market（FinMindClient + Fetcher，選填 FugleQuoteClient / FugleStreamClient）
    ├── indicator（Engine）
    │       └── store.CandleRepo
    ├── signal（Engine）
    │       ├── indicator.Engine
    │       └── store.{CandleRepo, SignalRepo, ChipScoreRepo}
    ├── chip（Syncer + scoring helpers，三大法人、融資融券、券商分點、chip_scores）
    │       └── market.FinMindClient + store chip repos
    ├── scheduler（cron jobs，daily_close 收盤後跑 SR zone 驗證；chip_daily_sync 另由傍晚獨立 cron 觸發；stock_symbol_sync 每日同步股票主檔）
    │       ├── market.Fetcher
    │       ├── market.StockSymbolSyncer
    │       ├── signal.Engine
    │       └── analysis.SRZoneVerifier
    ├── backtest（Manager，透過 Python 服務執行）
    ├── analysis（Client 呼叫 Python 計算：/analyze 與 /sr-zones 共用同一個
    │       Client；Verifier 與 SRZoneVerifier 都是純 Go 比對 candles 驗證，
    │       不呼叫 Python，見 sr-zone-scoring.md「十四」）
    └── api（Gin HTTP + WebSocket Hub + 前端靜態檔案）
            ├── middleware.Auth（JWT 驗證）
            ├── handler.Auth（register / login，新帳號預設 inactive）
            ├── handler.User（GET /users, PATCH /users/:id/status）
            ├── handler.{Candle, Indicator, Signal, Watchlist, Market, Backtest, Analysis}
            ├── handler.SRZone（store.SRZoneRepo，見 sr-zone-scoring.md）
            ├── handler.Chip（GET /chips/*, POST /chips/sync）
            └── ws.Hub
```

## 認證架構

```
Client
    ↓ POST /api/v1/auth/login（email + password）
Auth Handler
    ↓ bcrypt.CompareHashAndPassword
    ↓ 檢查 user.status == "active"（否則 → 403 Forbidden）
    ↓ jwt.NewWithClaims（HS256，TTL 24h）
    ← 返回 JWT token
Client
    ↓ Authorization: Bearer <token>（每個受保護的請求）
middleware.Auth
    ↓ jwt.ParseWithClaims → 驗簽 + 驗期限
    ↓ 注入 user_id / email 至 Gin context
Handler（受保護）
```

所有 `/api/v1/*` 路由除 `/auth/register` 和 `/auth/login` 外，皆需通過 JWT 驗證。

### 使用者狀態機

```
register → inactive ──→（管理員 PATCH /users/:id/status）──→ active ──→ 可登入
                                                                  ↓
                                                              inactive（停用）
```

- 新帳號一律 `inactive`，防止未授權使用者自行進入系統
- 第一個管理員需直接修改資料庫啟用（見開發指南）
- 後續帳號可由已登入的管理員透過前端「使用者管理」頁或 API 啟用

---

## 前端頁面

Svelte 單頁應用（`frontend/src/routes/`），登入後由 Sidebar 切換：

| 頁面 | Route | 說明 |
|------|-------|------|
| Dashboard | `dashboard` | 監控清單（即時報價/RSI/量比/趨勢/訊號/★監聽切換）+ K線圖（可疊加 MA5/MA20/MA60，個別開關）+ 訊號面板 |
| 個股分析 | `analysis` | 輸入股票代號觸發分析（`POST /analysis`），顯示支撐/壓力/進場/停損/停利，可對歷史分析手動重新驗證或刪除 |
| 支撐/壓力機率分析 | `sr-zones` | 輸入股票代號觸發 SR Zone Scoring（`POST /sr-zones`），顯示機率模型算出的區間、機率、EV/RR、可拆解交易分數；另有「訓練/更新機率模型」區塊（`POST /sr-zones/train`）。詳見 [sr-zone-scoring.md](./sr-zone-scoring.md) |
| Trade Analysis | `positions` / `trade-analysis` | 輸入股票代號產生 FLAT/LONG 共用交易決策；底層使用 immutable ledger、AVG projection、SR Zone snapshot 與 `position_analyses` 快照 |
| 歷史資料回補 | `backfill` | 勾選監控清單股票回補 K 棒（`POST /market/backfill`）；下方另有「手動計算指標」（`POST /indicators/:symbol/compute`）與「手動評估訊號」（`POST /signals/:symbol/evaluate`）兩個區塊，任意股票代號都可用 |
| 策略回測 | `backtest` | 送出回測任務（`POST /backtest`）、輪詢狀態、查看結果與逐筆交易 |
| 籌碼分析 | `chips` | 查詢籌碼摘要、歷史分數、券商分點排行，並可手動同步籌碼資料（`POST /chips/sync`） |
| 排程監控 | `scheduler` | 顯示 `pre_market`/`intraday`/`daily_close` 排程執行紀錄 |
| 使用者管理 | `users` | 啟用/停用帳號 |

Dashboard 的即時欄位以 REST 主動 hydrate（`/candles`、`/indicators`、`/signals`），
WebSocket 只在有新訊號時推播覆蓋，因為後端目前只會廣播 `signal` 事件。

WebSocket 訂閱不是整份監控清單，而是只對監控清單裡標記 `watched=true` 的
股票訂閱，且**同時最多 3 檔**（`store.MaxWatchedSymbols`，見
`PATCH /watchlist/:symbol/watch`）；`ws.Hub` 也有相同上限的防禦性檢查。
K線圖的 MA5/MA20/MA60 是前端用已載入的 candles 收盤價自行計算（rolling
window），不是另外呼叫 API，因為 `/indicators/:symbol` 只回傳最新一筆快照，
沒有整段歷史序列。

### 手動觸發端點（不限監控清單）

排程只會處理監控清單裡的股票；以下端點刻意設計成**任意股票代號**都能用，
用於補算/確認監控清單之外的股票（例如剛上市的 ETF、只是想先看看資料夠不夠）：

| 端點 | 說明 | 前提 |
|------|------|------|
| `POST /market/backfill` | 補歷史 K 棒 | 無 |
| `POST /indicators/:symbol/compute` | 手動算一次指標快照並寫入 | candles ≥ 35 根 |
| `POST /signals/:symbol/evaluate` | 手動跑一次訊號判斷（內部會先呼叫指標計算） | candles ≥ 35 根 |

三者都在「歷史資料回補」頁面有對應 UI，也可以直接呼叫 API（見
api-reference.md）。

## 兩個標的清單：`watchlists` 與 `evaluation_universe`

系統有**兩份彼此獨立的標的清單**，職能完全不同。搞混會導致嚴重的成本誤判，
所以先講為什麼不合併。

### 為什麼不能合併

`watchlists` 驅動**六個**流程，每加一檔就同時乘上六份成本：

| 流程 | 觸發頻率 | 每檔成本 |
|---|---|---|
| `runIntradayJob` | 盤中**每 5 分鐘** | 1 request |
| `runPreMarket` | 每日 08:50 | `BackfillHistory(5 天)` |
| `RunDailyClose` | 每日 15:00 | 1 request ＋ signal 評估 |
| `runChipDailySync` | 每日 21:00 | **2 requests**（法人＋融資券） |
| `runSRZoneVerification` | 每日盤後 | SR zone 驗證計算 |
| SR evaluation 排程 | `symbols: []` 時 | replay 母體 |

盤中那一列是決定性的：FinMind 節流是 5 req/min，把 135 檔放進 watchlist
會讓**光是一輪盤中掃描就要 27 分鐘**，而它應該每 5 分鐘跑完一次。
這不是「比較慢」，是**根本不可行**。

所以研究用的標的池必須與 watchlist 分離。

### 現況職能

| | `watchlists`（11 檔） | `evaluation_universe`（135 檔） |
|---|---|---|
| 盤中分 K | ✅ 每 5 分鐘 | ❌ |
| 盤前補缺口 08:50 | ✅ | ❌ |
| 收盤日 K ＋ signal 15:00 | ✅ | ❌ |
| **日 K 維護 16:00** | ❌ | ✅ **唯一職能** |
| 籌碼同步 21:00 | ✅ | ❌ |
| SR zone 驗證 | ✅ 每日盤後 | ❌ |
| **SR zone production 分析** | ✅ **平日 17:00 ＋ 22:00** | ❌ |
| 排程版 SR evaluation 的母體 | ✅ | ❌ |
| **公司行動同步 06:30** | ✅ **每個排程日全量** | ⏳ 分片輪替（見下） |

**production 分析排程（2026-08-20 起）**：`watchlists` 每檔每交易日跑**兩輪**帶身分追蹤的
SR zone 分析——17:00 那輪拿到的是前一日籌碼，22:00 那輪（晚於 21:00 的籌碼採集）才有當日的。
它是 `stock_sr_zone_analyses` 的 production 母體來源，也是 [`todo.md`](./todo.md) T-049 前置①
（新舊兩套 active 事件集合逐日並行比對）的前提。
⚠️ **它不是 decision replay 的前提**（2026-09-01 更正）——replay 讀的是 `candles`
（`run_decision_replay()` → `_load_db_sources()`），不讀 `stock_sr_zone_analyses`。
預設關閉，細節見 [`api-reference.md`](./api-reference.md) 的
`POST /scheduler/sr-analysis/run`。

### 日 K 維護（`evaluation_universe_sync`，平日 16:00）會跳過「今天已有日 K」的標的

這支排程逐檔對池內標的呼叫 `BackfillHistory(days)`，在 FinMind 的 5 req/min 下
135 檔約需 **27 分鐘**。它取到池成員之後、送請求之前會做**兩道過濾**，順序固定：

| 順序 | 過濾 | 依據 | 計數欄位 |
|---|---|---|---|
| 1 | 剔除**已下市**的池成員 | `StockSymbolRepo.StatesBySymbols(池成員)` 的 `is_listed` | `delisted` / `stock_symbol_unknown` |
| 2 | 剔除**今天已有日 K**的標的 | `CandleRepo.SymbolsWithCandleOn(池成員, "1d", 台北當日)` | `skipped` |

（第 1 道 2026-08-27 起，原記於 `issue.md` I-094；第 2 道 2026-08-25 起，原記於
`todo.md` T-062，兩者都已收斂。）

**順序不能換。** 反過來的話，已下市但今天剛好被跳過的標的不會進入下市計數，
`delisted` 會隨當日的跳過情況浮動，看的人無從判斷池裡到底累積了多少死標的。

#### 下市過濾（第 1 道）：判定是三態，不是布林

`evaluation_universe.active` 與 `stock_symbols.is_listed` 是**兩份獨立維護的清單**——
前者由選池流程維護，後者由 `stock_symbol_sync`（每日 06:30）自 TWSE 清冊同步，
兩者之間沒有任何連動。少了這道過濾，下市標的會每天被發一個註定拿不到資料的請求並記
`success`，而且數量隨時間累積。

| 主檔狀態 | 處置 | 計數 |
|---|---|---|
| `is_listed = true` | 保留，照常回補 | — |
| `is_listed = false` | **過濾** | `delisted` |
| **主檔查無該 symbol** | **保留**（fail-open） | `stock_symbol_unknown` |

⚠️ **第三態不能省，而且缺席語意要在呼叫端寫死**：`StatesBySymbols` 回傳的 map 裡
**沒有那個 key 是「主檔還沒收錄」，不是「已下市」**。新入池的標的可能還沒被
`stock_symbol_sync` 收錄，主檔同步失敗那天也會整批查無。兩者的處置相反
（前者保留、後者過濾），**寫錯不會有任何東西報錯**——標的會靜默停止更新，
那正是下方「日 K 缺漏偵測」要消滅的失敗模式。

**查詢本身失敗時維持全量回補**（記 Error log），降級方向與第 2 道一致：
「多抓一點」可接受，「靜默少抓」不可接受。

**刻意不採「一次性把 `evaluation_universe.active` 設 false」**：那會在主檔誤判
（例如某天清冊抓取不完整）時**靜默清掉池成員**，而重新入池是人工動作。
每輪重新過濾則是可逆的——主檔隔天恢復，抓取就自動恢復，這由
`TestEvaluationUniverseSyncResumesAfterRelisting` 釘住。

**`StatesBySymbols` 回傳值帶 `market` 不只帶布林**：這是刻意的，下方「日 K 缺漏偵測」的
個股核對要靠它決定打上市還是上櫃端點（實測池內上市 101 / 上櫃 34）。只回布林會逼
偵測再查一次主檔，或退回逐檔查詢——而逐檔查詢在 135 檔就是 N+1，
正是這支批次方法要消滅的東西（`List` 會載入整份主檔，實測 49,458 列）。

**查詢一定要把池成員帶進去。** `candles` 的複合索引是 `(symbol, timeframe, ts)`，
symbol 是首欄；只約束 `timeframe` 與 `ts` 的話 PostgreSQL 16 沒有 skip scan 可用，
會退化成整張 `candles` 的 seq scan。live 實測（2026-08-25 review）：

| 查詢 | 計畫 | 耗時 | buffers |
|---|---|---|---|
| 只限 `timeframe` ＋ `ts` | Parallel Seq Scan（掃掉 211,837 列） | 368 ms | 13,814 |
| 加上 `symbol IN (池 135 檔)` | **Index Only Scan**（Heap Fetches: 0） | **1.96 ms** | 409 |

呼叫端本來就有那份清單，要問的問題也正是「**這些**標的今天抓過了沒」，
所以這不需要新增索引。

**為什麼跳過是安全的**：跳過的前提是「已存在的當日日 K 是定案值」。池內標的
**不進盤中掃描**（見上方職能表），所以它們的當日日 K 只有兩個來源——
`daily_close`（15:00，只涵蓋與 watchlist 重疊的那幾檔）與這支排程自己（16:00），
兩者都在收盤後。不存在「跳過了一根還會變動的盤中值」。

> ⚠️ **這個論證依賴「池不進盤中」這條分工。** 若日後把池接進盤中掃描，或任何會在盤中
> 寫入日 K 的流程涵蓋到池成員，這個最佳化就不再安全，而且**不會有任何東西報錯**。
> 要動那條分工之前，先回頭改 `dropSymbolsSyncedToday`。

**這是效率與配額的最佳化，不是資料正確性的守門**：`BackfillHistory` 抓的是
`today-days ~ today`（`days` 預設 10）且 `BulkInsert` 走 upsert。跳過機制解的是
「中斷後補齊要重跑整輪 27 分鐘」與「池成長後每輪都全量重抓」。因此
**查詢失敗時退回全量抓取而不是整輪放棄**——降級方向刻意選「多抓一點」，少抓會靜默漏標的。

> ⚠️ **跳過會關掉一個原本存在的自癒性質**（2026-08-26 更正，原記於 `issue.md` I-091）。
> 這裡原本寫著「某天漏掉的 K 棒**隔天那輪本來就會補回來**」——那句話在導入跳過之後
> **只對尾端成立**：
>
> * **尾端缺口（今天沒抓到）會自癒**：該檔沒有當日 K 棒 → 不會被跳過 → 隔天連同
>   10 天視窗一起重抓。
> * **視窗中間的缺口不會自癒**：只要該檔**今天有** K 棒就會被整檔跳過，
>   五天前那個洞永遠不會被重新抓取。導入跳過之前，每檔每天都重抓 10 天視窗，
>   任何 10 天內的洞都會被 upsert 補平——那個性質已經沒有了。
>
> 這是 T-062 的副作用，**當時沒有識別到**。缺口偵測因此不能只看「本輪筆數」或
> 「最新日期」，必須對整池的**實際日期集合**掃缺洞——那就是下一節的
> `candle_gap_detection`（原記於 `issue.md` I-091）。

#### 日 K 缺漏偵測（`candle_gap_detection`）

**要解的問題**：上面那支排程的 `success` 只代表「請求沒失敗」，不代表「拿到了該有的
資料」。2026-08-25 那輪 135 檔全成功、`symbols_failed=0`，其中 `2867` 只回了 3 根
（視窗內有 7 個交易日），**沒有任何東西報錯**。

**沒有自己的 cron**：掛在 `runEvaluationUniverseSync` 尾端（回補之後才有意義），
但寫**獨立的 `job_runs` 紀錄**，比照 `sr_zone_verify` 掛在 `daily_close` 尾端的既有 pattern
——偵測判 `partial` 不會污染回補的狀態。用自己的 `context.WithTimeout`，
**不沿用回補的 ctx**：回補逾時不該讓偵測連帶失效。**預設關閉。**

⚠️ **兩個開關在 cron 路徑上是巢狀的，在手動路徑上不是**——這兩條要分開讀：

| 路徑 | 生效條件 |
|---|---|
| **自動 cron** | **兩個開關都要開**。偵測的註冊寫在 parent 的 `if evaluationUniverse != nil && evaluationUniverseCfg.Enabled` 區塊裡（`scheduler.go:305`），所以 `EVALUATION_UNIVERSE_ENABLED` 沒開時，只設 `CANDLE_GAP_DETECTION_ENABLED=true` 不會讓它自動跑 |
| **手動觸發 parent** | **繞過 parent 的 `Enabled`**。`POST /api/v1/scheduler/evaluation-universe-sync/run` → `RunEvaluationUniverseSync()` → `runEvaluationUniverseSync()`，後者**只檢查 `evaluationUniverse == nil`**（`:1009-1011`），不看 `Enabled`；尾端照樣呼叫 `runCandleGapDetection()`（`:1105`）。而偵測的有效啟用條件 `candleGapDetectionEnabled()` ＝ **自身開關 && 四項依賴**（`candle_gap_detection.go:63-65`），**不含 parent 開關** |

**所以 parent 關閉、偵測開啟、依賴齊全時，手動觸發 parent 仍會執行偵測並寫入
`job_runs`。**

⚠️ **`disabled` 狀態沒有啟動錯誤可查**：「已啟用但依賴不齊，不註冊」那條 Error log
**只在 parent 已註冊、偵測自身 enabled、但四項依賴缺一時**才出現（`scheduler.go:316-322`），
只開偵測那個開關的話照它排錯會一無所獲。

⚠️ **「不執行、不寫任何 `job_runs`、沒有痕跡」只成立於兩種情形**：完全沒有手動觸發的
自動排程情境，或**偵測自身未啟用／依賴不足**（此時 `runCandleGapDetection` 在第一行早退）。
在這兩種情形下很容易被誤讀成「沒有缺口」。

在 dev 驗它的完整前置見
[`development-workflow.md`](./development-workflow.md)「在 dev stack 上驗排程類功能」。

##### 資料流

```text
池成員（ListActive）
  → StatesBySymbols（is_listed / market，缺席＝unknown）
  → 實際日期集合（CandleRepo.CandleDatesInRange，單一查詢）
  → 預期交易日（TWSE 年度日曆，date= 參數，驗證年份）
  → 差集 = 候選缺口
  → 對照源陳舊度：lag >= market_stale_days → 整批 unavailable
  → 缺漏日期晚於 source_as_of → deferred（不告警、不算失敗）
  → 按 (market, missing_date) 分組；比例達標 → aggregate 告警，不逐檔
  → 其餘：公平排序掃描 → 逐檔核對（TWSE STOCK_DAY／TPEx tradingStock）
  → 依 (symbol, timeframe) coalesce → RecordAttempts
  → finishRunDegraded(partial) ＋ error 以 "; " 合併
```

##### 三種結論必須分得開

| 結論 | 條件 | job 狀態 |
|---|---|---|
| **正常** | 交易所那天也沒有成交（停止買賣／下市／休市） | `success` |
| **確認缺口** | 交易所那天**有成交**、我們沒有 K 棒 | `partial` ＋ `upstream_data_gap` |
| **驗不了** | 對照源失敗／格式變動／回應歸屬對不上／breaker 開啟／主檔查不到 | `partial` ＋ `verification_unavailable` |

⚠️ **把第三種記成第一種，這個機制就在最需要它的時候靜默失效**——那正是它要消滅的
問題形狀，只是換成發生在偵測器自己身上。所有讀不懂的回應一律 `verification_unavailable`，
**不猜測、壞在明處**。

⚠️ **`symbols_failed` 固定 0**：缺漏與不可用都不是「標的失敗」——那些標的的回補本身是
成功的，上游回什麼就寫什麼。失敗的是「上游該給而沒給」。詳見
[`api-reference.md`](./api-reference.md) 的 `partial` 第四種成因。

##### 預期集合來自年度日曆，不是市場層級端點

**實際日期集合只能回答「有哪些天」，回答不了「少了哪些天」**——沒有預期集合就沒有缺口。

預期集合用 TWSE 的 `holidaySchedule`（整年預先公布，**不會停滯**），
市場層級端點只用來算 `source_as_of` 與落後交易日數。

⛔ **不能拿市場層級端點的最後日期當稽核右界**：它若回應成功、格式正常、內容卻停滯數日，
視窗會跟著倒退，**所有比它新的缺口都不會被檢查，而且不會被歸類為驗證不可用**。

⛔ **年度日曆不是「休市日清單」，不能集合相減**。實測 115 年度至少四種列型，
兩個方向都會錯：

| 列型 | 實例 | 是不是交易日 |
|---|---|---|
| 放假休市 | 1/1 開國紀念日、9/25 中秋節 | ❌ |
| **正常交易日的標記** | 1/2 開始交易日、2/11 最後交易日、2/23 春節後開始交易 | ✅ **是** |
| **市場無交易，僅辦理結算交割** | 2/12（四）、2/13（五） | ❌ |
| 週末列 | 2/28（六） | ❌ |

全部扣除 → 1/2、2/11、2/23 這三個真正的交易日被排除；只扣「放假」→ 2/12、2/13 被漏掉
（它們是**平日、不是放假日，但市場無交易**）。所以規則是**逐列分類**，未知列型別一律
`verification_unavailable`——這是中文字串比對，本質脆弱，那個降級方向就是為了讓
TWSE 改字時**壞在明處**。

**兩市場共用同一份日曆**（具名假設）：台灣兩市場的開休市日實務上一致。
若日後出現單一市場臨時休市，該市場當天會整批落進 aggregate 告警而不是逐檔誤報——
那是可接受的降級。

##### 陳舊與 deferred

`lag = |{ 交易日 d : source_as_of < d <= expected_last_trading_day }|`（**左開右閉**）。

* **單位是交易日不是日曆日**：週五的資料到週一才更新，日曆日差 3、實際只落後一個交易時段。
  用日曆日會讓每個週一都誤報。
* **比較是 `lag >= market_stale_days`**（不是 `>`）：寫成 `>` 會在剛好等於門檻時漏報。
  預設 2 的語意是「容忍一個交易日的發布延遲」。
* 缺漏日期**晚於** `source_as_of` → **`deferred`**：對照源還沒發布那天，「查不到」不等於
  「無成交」。不告警、不更新 `last_verified_at`、不加失敗計數，但**更新
  `last_attempted_at`**（確實嘗試過，排序要前進），且**不讓該輪 degraded**。
  它有時間上界——持續落後到 `lag >= market_stale_days` 就會升級成 `verification_unavailable`。

##### 請求量的三道防線

**最該被抓到的情境剛好也是請求量最大的情境**：上游某天整批漏給 → 全池都有候選 →
逐檔逐月查個股端點 → 偵測本身變成對交易所的壓力測試。

1. **aggregate 短路**：按 `(market, 缺漏日期)` 分組，該組 distinct symbol 數 ÷ 該市場有效池
   達到 `aggregate_ratio` → 發一次來源級告警，**不展開逐檔**。
   ⛔ 判定維度不是「缺口總筆數」——不同日期的零星缺口累加也會過門檻。
   ⛔ 有效池 < `aggregate_min_symbols` 時**強制逐檔**：單檔市場那一檔合法停止買賣，
   比例就是 100%，會被短路成來源級告警而違反「`2867` 不得告警」。
2. **`candidate_cap_per_run`**：單位是**候選標的數，不是 HTTP 請求數**。一檔的所有月份
   一次做完才扣一個額度——按月份扣會讓 cap 20 在跨月時只處理 10 檔，更嚴重的是可能
   **在同一 symbol 的月份中途停止**，拿一半的結果去做跨月 coalesce。
3. **`request_interval_ms` ＋ 來源層級 breaker**。breaker 是**行程內**的 runtime 安全閥，
   與逐 symbol 的 `consecutive_failures` 是**兩層獨立的計數**——同一來源對五個 symbol
   各失敗一次，逐 symbol 都是 1，推導不出「這個來源已連敗五次」。
   ⚠️ **只有真的送出且失敗的請求才累加 breaker**：能力限制與 `deferred` 沒有發出失敗請求，
   算進去等於用一個已知限制去癱瘓一個健康的來源。

##### 公平性

候選排序鍵是 **(沒有 state 者優先, `last_attempted_at` 由舊到新, symbol)**，
**掃描而非預先截斷**——先截斷的話，處理到一半某來源才斷路，剩下的只會被跳過而
其他來源的候選不會遞補，該輪等於只驗了幾個。

⚠️ **排序鍵是 `last_attempted_at` 不是 `last_verified_at`**：只在成功後更新的話，
持續失敗的前 `cap` 個會永遠最舊、每輪繼續佔滿配額，後面的候選永遠輪不到。

在「breaker 未開啟」且「state 寫入成功」的前提下，**任一候選最遲在第 `ceil(N/cap)` 個
排程週期被嘗試**。兩個前提失敗時都會讓該輪 `degraded`，所以**停滯是看得見的**。

⛔ **只要有任何候選因 breaker 被跳過，該輪就是 `partial`**——否則「部分驗不了但其他驗成功」
會顯示 `success`，又變回這支排程要消滅的形狀。

**持續存在的缺口每輪都會回報**，這是刻意的：把它壓成一次性通知，等於讓一個仍然存在的
問題消失。

##### 驗收實測（2026-08-28）

2026-08-28 14:37 隨 `CANDLE_GAP_DETECTION_ENABLED=true` 上線。**正向（抓得到真缺口）與
負向（合法無成交不誤報）兩半都驗過**——只驗正向會漏掉誤報，只驗負向會漏掉漏報。
兩半的證據來源不同：負向來自 live 首輪的真實停止買賣標的，正向來自 dev 人工造缺口。

**live 首輪（16:25:13 → 16:25:19，5.8 秒）**

| 欄位 | 值 |
|---|---|
| `job_runs` | `success`、`symbols_total=135`、`symbols_failed=0`、`error` 空 |
| log | `pool=135 candidates=6 gap=0 unavailable=0 deferred=0 breaker_skipped=0 source_as_of=2026-08-28` |
| `candle_verification_state` | 1 列：`2867 / 1d / verified / consecutive_failures=0` |

6 個候選**全部**來自 `2867`：它最後一根日 K 是 08-19（08-20 起停止買賣），在視窗內缺 6 天。
逐檔核對後交易所那邊也沒有成交，判 `verified`、不計入 `gap`、不告警——
這就是「合法停止買賣不得誤報」的天然負向案例。請求量方面，6 個候選收斂成 1 檔逐檔核對，
遠低於 `candidate_cap_per_run=20`。

⚠️ **視窗是「到前一交易日為止」的 N 個交易日，不含當日。** 該輪視窗是
08-14、17、18、19、20、21、24、25、26、27（`lookback_trading_days=10`），
`2867` 缺其中 6 天——`candidates` 是 6 而不是 7 就是這麼來的。判讀這個數字時要記得。

**dev 造缺口（正向，池縮成三檔）**

`2330` 目標／`1101` 對照／`2867` 負向，刪掉 `2330` 視窗中段的 08-21：

| 觀察 | 結果 |
|---|---|
| `job_runs` | **`partial`**、`error` = **`upstream_data_gap: 1 筆缺漏已確認`** |
| log | `pool=3 candidates=7 gap=1 unavailable=0 deferred=0` |
| `2330` | `last_result=gap` |
| `2867` | `last_result=verified`（負向案例在 dev 重現一次） |
| `1101` | **無 state 列**——完整標的不會進候選集合，那是設計如此，不是漏寫 |

`candidates=7` ＝ `2330` 的 1 天 ＋ `2867` 的 6 天；`gap=1` 只認人工那一筆。
⛔ **判準是 `error` 的前綴**：`upstream_data_gap` 才算驗證通過，
`verification_unavailable` 代表根本沒驗成。

**跳過機制與人工缺口的交互，是驗收設計的關鍵**：那一輪 `skipped=2`
（`2330` 與 `1101` 都有當日 K）。`runEvaluationUniverseSync` 的順序是
`dropSymbolsSyncedToday` → `BackfillHistory(days=10)` → 偵測，而回補走 upsert，
所以**目標必須落在跳過那批**，否則人工缺口會在偵測跑到之前被補回去、偵測什麼也看不到。
那不是實作壞了，是驗收步驟設計錯了。

**尚未驗到的**：live 端的**正向**告警（真實上游漏資料）至今沒有發生過，
所以 `partial` ＋ `upstream_data_gap` 在 live 的表現只有 dev 的等價證據；
`deferred`、陳舊升級與 breaker 三條路徑也都還沒在真實流量下被觸發。
**這份清單的權威版本在下一節**（「live 運作觀察」的「仍未在 live 觸發的四條路徑」），
截至 2026-09-01 連續三個交易日仍然一條都沒觸發——不要在兩個地方各自宣稱進度。

##### live 運作觀察：三個交易日（2026-08-28 / 08-31 / 09-01）

上線後連續三個交易日的 16:25 輪次**全部 `success`**，`error` 欄皆空、無 `partial`：

| 交易日 | status | 檔數／失敗 | 起訖（CST） | 耗時 |
|---|---|---|---|---|
| 2026-08-28（五） | `success` | 135 / 0 | 16:25:13→16:25:19 | 5.81 秒 |
| 2026-08-31（一） | `success` | 135 / 0 | 16:25:18→16:25:24 | 6.21 秒 |
| 2026-09-01（二） | `success` | 135 / 0 | 16:25:08→16:25:14 | 5.69 秒 |

三輪的 `candle_verification_state` 全表都只有 **`2867` 一列**，判 `verified`、
`consecutive_failures=0`。**耗時 5.7～6.2 秒的差異是執行時間波動，不是請求數變化**——
缺漏日期變多不會增加請求數（理由見下）。

**請求量可以事前算出來，不必靠 log。** 兩條規則決定它：

1. **視窗 ＝ 從「昨天」往回數 `lookback_trading_days`（預設 10）個交易日**，不含當日
   （`candle_gap_detection.go:388-393`）。
2. **候選依 `(symbol, market, year, month)` 去重**（`:557-575`）——交易所的個股端點按月回傳，
   同一檔同一個月的多天缺口只需要一次請求。

以 2026-09-01 那輪為例：視窗是 08-18～08-31 共 10 個交易日；`2867` 的日 K 止於 08-19，
所以候選是 08-20/21/24/25/26/27/28/31 共 **8 天，全部落在 8 月** → 個股核對 **1 次請求**。
`candidate_cap_per_run=20`（單位是候選標的）遠未逼近。

⚠️ **兩條規則的證據等級不同，不要混為一談**：

* **規則 1（視窗）有實測反向驗證**：08-28 那輪視窗是 08-14～08-27，依規則算出 `2867`
  缺其中 6 天，與該輪 log 的 `candidates=6` 完全吻合。
* **規則 2（月份去重）沒有被直接量到**。`candidates=6` 數的是**缺漏日期**，不是送出的
  HTTP 請求數；三輪的 log 都沒有記請求數。目前的「1 次請求」是由
  `groupCandidatesBySymbol` 的分組邏輯 ＋ `2867` 被判 `verified`（代表核對確實發生過）
  **推導**出來的，不是實測值。要真的量到，得另外加一個請求計數或在該輪的 log 保留期內取證。

⚠️ **2026-09-02 起請求數會從 1 變成 2，那是預期不是惡化。** 該日視窗變成 08-19～09-01，
**納入 09-01 這個 9 月的日子**，於是 `2867` 的缺漏日期跨月，(symbol, month) 去重後
變成兩組。

⚠️ **但它不會一路累加下去**（2026-09-01 review 修正——前一版寫「之後每跨一個月就會多一組」，
那是錯的）。**一般規則是：某一檔的請求組數 ＝ 該檔的缺漏日期在滾動視窗內的
distinct `(year, month)` 數**，視窗滾過去之後舊月份就不再計入。

以 `2867` 這次為例，確定的軌跡是 **1 → 2 → 1**：09-02 視窗同時含 8、9 月 → 2 組；
等 8 月的日期全部滾出視窗 → 回到只剩 9 月的 1 組。

⛔ **不要寫成「上限固定是 2」。** `expectedTradingDays()` 只是不斷往前找滿
`lookback_trading_days` 個交易日（`:388-400`），**程式裡沒有任何「最多橫跨兩個月份」的
限制**；若中間某個月因長假而交易日極少，理論上 10 個交易日仍可能橫跨三個月份。
除非另外有交易日曆的 invariant 保證，否則只該宣稱「不會永久累加」。

⚠️ **`docker logs` 在這台機器不能當成運作觀察的證據來源。** 2026-08-31 實測：偵測跑完
4 分鐘後三個 `stock_trading-*` 容器於 16:29:40 被重新建立，backend 內只剩重建後的數行 log，
當天已無法用 `docker logs ... | grep 'candle gap detection done'` 取證。
**判定一律以 `job_runs` 與 `candle_verification_state` 兩項 DB 證據為準**，它們比 log 耐久；
要取 log 佐證，只能在該輪跑完到下次容器重建之間。

**仍未在 live 觸發的四條路徑**（截至 2026-09-01，三個交易日皆未發生）：

| 路徑 | 現有證據 |
|---|---|
| 正向告警（`partial` ＋ `upstream_data_gap`） | 只有 dev 人工造缺口的等價證據 |
| `deferred`（缺漏日期晚於 `source_as_of`） | **無** |
| 陳舊升級（`lag >= market_stale_days` → 整批 `unavailable`） | **無** |
| breaker 開啟後跳過 | **無** |

⛔ **這四條列在這裡是為了不要被當成「已驗過」。** 三個交易日的 `success` 只證明了
**「沒有真實上游資料缺口時不誤報」**與「合法停止買賣判 `verified`」這兩件事。

⚠️ **不要寫成「沒有候選缺口」**（2026-09-01 review 修正）：三輪**都有** `2867` 的候選缺漏
（8～9 天），只是逐檔核對後交易所那邊也沒有成交，所以判 `verified` 而不告警。
**候選缺口一直都在，被驗掉的是「它是不是真的上游漏資料」。**

##### 相關

* 表結構與 repo 契約：[`database-schema.md`](./database-schema.md) `candle_verification_state`
* job 狀態與 `partial` 語意：[`api-reference.md`](./api-reference.md)
* 參數與合法範圍：`backend/config.yaml` 的 `candle_gap_detection` 區段

##### live 實測：日常節省遠小於預期（2026-08-26 首次以新版執行）

| 輪次 | 版本 | log | `job_runs` | 耗時 |
|---|---|---|---|---|
| 2026-08-25 16:21 | 舊版（全量） | — | `success`，`symbols_total=135` | **26.9 分** |
| 2026-08-26 16:00 | **新版（跳過）** | `pool:135 attempted:126 skipped:9 failed:0` | `success`，**`symbols_total=135`** | **25.2 分** |

⚠️ **日常只省約 1.7 分鐘（9 檔），不是「27 分鐘 → 只補缺口」。**
16:00 開跑時，池內**唯一可能已經有當日 K 棒的**就是與 watchlist 重疊的部分，
而**11 檔 watchlist 全部都在池內**——上限就是 11 檔。

**跳過數會浮動，不是固定 11**：2026-08-26 的 `daily_close`（15:00）雖然記
`success 11/0`，但只有 9 檔的 K 棒在 15:00:09～15:02:00 寫入；
`3630` 與 `5490` 要到 **16:16 / 16:19**（池同步自己跑的時候）才拿到——
上游那時才發布。所以跳過數取決於**上游當日的發布時間**。

**這不表示 T-062 沒有價值，而是價值來源要講準**：它真正解的是
「**中斷後補齊要重跑整輪**」（2026-08-25 那次中斷後，手動重跑會白抓已完成的 54 檔）
與「池成長後每輪全量重抓」的線性成本。**日常穩態的節省本來就有 11 檔的上限。**

`symbols_total` 兩輪都是 135，確認池大小語意維持正確（見上）。

**`symbols_total` 仍然是池大小，不是本輪實際抓取數。** 換成抓取數會讓
`/scheduler/status` 的數字每天不同（135 / 81 / 0 …），看的人無從判斷哪個才是異常。
實際跑了多少要看 log 的 `evaluation universe sync done`，它帶
`pool` / `attempted` / `skipped` / `failed` 四個欄位。

**要注意的讀法**：池內資料都已最新時，那一輪是 `attempted=0`、`failed=0`、
`symbols_total=135`、狀態 `success`——「135 檔全部成功」但一個請求都沒送。
語意上是對的（池內資料確實都是最新），判讀時要看 log 的 `skipped` 才知道發生了什麼。

### 公司行動同步是唯一「watchlist 優先、其餘輪替」的排程

`corporate_action_sync`（平日 06:30）的標的來源是 **`candles` 內所有相異 symbol**
（約 857 檔，涵蓋兩份清單以外的歷史標的），但它**不是每天全跑**：

* **watchlist 全量**——每個排程日都同步。還原係數過期會直接影響訊號與 SR 分析，
  這批是實際拿來做交易決策的。
* **其餘標的分片輪替**——依 `fnv32a(symbol) % shard_count` 決定片別，
  當日跑第 `(週序號×5 ＋ 平日序號) % shard_count` 片。預設 `shard_count: 5`，
  即週一到週五各一片，**每檔每週至少覆蓋一次**。

**watchlist 讀不到時降級成「只跑當日分片」**，不是整輪放棄：分片那一批與 watchlist 無關，
讓它們陪葬會多掉一整片，而片號由日期決定、沒有游標，掉的那片要等下一輪（預設一週）
才輪得回來。代價是 watchlist 標的那天沒同步，所以該輪一律記成 `partial`（即使分片內零失敗）
並在 `error` 欄寫明——語意見 [`api-reference.md`](./api-reference.md)。
log 的 `watchlist_degraded=true` 可以把它與「watchlist 真的是空的」分開。

**為什麼要分片**：逐檔同步要打 Yahoo（除權息）與 FinMind（減資），節奏由較慢的
FinMind 5 req/min 決定——**每檔約 12 秒**，857 檔要約 3 小時。在此之前預算是 10 分鐘，
於是每天固定只跑得完排序最前的約 50 檔，**8xxx／9xxx 那段永遠輪不到**，
而且逾時失敗完全不被回報（2026-08-24 修）。分片把「每天跑同樣的前 50 檔」
換成「181 檔／天、每檔每週輪到一次」，落在 45 分鐘預算內。

**為什麼不是調高 `finmind.rate_limit`**：全 repo 只有一個 `FinMindClient` 實例，
limiter 是共用的——調高會同時放大籌碼同步的請求速率，而 5/min 是綁帳號等級的保守值、
額度未知。

**Yahoo 那半的速率來自 `yahoo.rate_limit`（預設 20/min），與 `yahoo.enabled` 無關**：
除權息客戶端是無條件建立的，且與盤中報價客戶端**共用同一個節流器**（同一個 host，
各自節流會讓實際速率加倍）。目前 FinMind 的 5/min 主導節奏，但**把 `yahoo.rate_limit`
調到 5 以下就會換邊**，45 分鐘預算跟著失準——那段設定看起來像「盤中專用」，實際不是。

**片號用 symbol 的 hash 而不是排序位置**：清單來自
`SELECT DISTINCT symbol FROM candles ORDER BY symbol`，順序穩定但**位置會漂移**——
新股上市會讓它後面的所有標的整批位移一格，被推過當天那片的標的要再等一輪。

**週序號取自 1970-01-05（週一）起算的連續週數**，不是 ISO 週數：後者跨年會從 52 跳回 1，
那個不連續會讓某片被跳過或連跑兩次（`shard_count > 5` 時才看得出來）。
週六日（只可能來自手動觸發）沿用當週週五那片，重算是冪等的。

**沒有交易日守衛，而且是刻意的**：除權息與減資是**已公告的排程**，不依賴今天是否開盤，
國定假日照跑既無害也不會漏抓。這與 `chip/sync.go`、`ListTradingDays` 那種
「從 candles 有沒有資料反推交易日」的慣例不衝突——那些是在判斷「某天有沒有行情資料」，
本 job 不需要那個判斷。**不要順手補上守衛**：那只會讓假日當天的那一片被整片跳過。

**某天沒跑到就掉一整片，這是刻意接受的**：片號由日期推導、**不持久化游標**，
所以容器停機、部署或 crash 讓某天的排程沒跑到時，那一片不會被補，要等下一輪
（預設一週）才輪得回來。判定為可接受——事件表是 upsert、還原係數重算是冪等的，
延一週只會讓那批標的的係數晚點更新，不會產生錯誤資料，而持久化游標的成本高於收益。
要確認有沒有掉片，看 `job_runs` 有沒有該 weekday 的紀錄。

**同理，某片長期跑不完的話，尾段仍然會「永遠輪不到」**：被預算 `break` 掉的那批，
下一輪還是同一批（片號由日期決定，沒有游標會接著跑），原本的覆蓋率破洞會以 1/5 的規模
復現。這不是靜默的——`job_runs` 會持續顯示 `partial` 並帶未處理檔數，看到就調
`shard_count`。

調參與旺季補跑（`shard_count: 1` 等於每天全量）見 `backend/config.yaml` 的
`corporate_action` 註解；`job_runs` 的狀態語意見 [`api-reference.md`](./api-reference.md)。

**`evaluation_universe` 目前只做一件事：讓 135 檔的日 K 保持新鮮。**
它不做任何分析，也**不參與任何交易決策或狀態推導**。

在它上線前，每次跑 evaluation 都得先手動回補一次對齊尾端——實測 2026-08-17
全庫只有 9 檔（皆為 watchlist 成員）有當日資料，池內其餘標的停在三天前。
尾端不齊會讓 evaluation 的「最後 N 根」視窗錯開，同一份報告隔幾天重跑得到不同結果，
且分不清是策略變了還是資料窗變了。池把這件事自動化了。

### SR 分析的兩個時段共用一個執行所有權

`sr_analysis`（17:00）與 `sr_analysis_chip`（22:00）是兩支獨立的 cron，
但**執行層只有一個所有權**：任一輪在跑時，另一輪不論由 cron 或手動端點觸發都不會執行。

⚠️ **資料層的時段冪等與執行層的併發是兩件事，混在一起就會改錯。**

| | 資料層（`srAnalysisSkipReason`） | 執行層（所有權） |
|---|---|---|
| 問的問題 | 這一輪對這檔還有沒有事情要做 | 現在能不能開始跑 |
| 兩輪的規則 | **刻意不同**（17:00 看今日 K 棒分析過沒；22:00 看當日籌碼入庫沒） | **共用一個** |
| 粒度 | 逐檔 | 整輪 |

**為什麼執行層必須共用**：這台 host 只有 2GiB，逐檔的峰值等同使用者手動點一次分析。
舊版是兩個 `atomic.Bool`、註解寫「兩輪之間互不影響」，與序列執行的前提直接衝突；
cron 相隔五小時撞不到，但**手動端點可以讓兩輪真的並行**。

**所有權的實作**：一個 mutex 保護一個字串（`srAnalysisRunningJob`），
「是否執行中」與「誰在跑」是同一個變數。⛔ **不要改回「`atomic.Bool` ＋ 另一個變數存持有者」**
——CAS 成功到寫入持有者之間有窗口，釋放順序不嚴謹還會清掉下一輪的持有者。

**三個入口，責任分明**（釋放只由一個層級負責）：

| 入口 | 誰用 | 取得 | 釋放 |
|---|---|---|---|
| `RunSRAnalysis(withChip)` | cron | 自己 | 自己 `defer` |
| `TryStartSRAnalysis(withChip) (running string, started bool)` | API handler | 自己，**同步** | 由它 spawn 的 goroutine `defer` |
| `runSRAnalysisOwned(ctx, withChip)` | 上面兩者 | **不取得** | **不釋放** |

**被擋時要看得見**，因為丟掉的那一輪**不會在隔天自動補回來**
（隔天 17:00 分析的是下一根日 K，不會補建前一天缺少的 chip-round 分析）：

* **cron 被擋** → 寫一筆 `failed` 的 `job_runs`，`error` 欄位帶持有者名稱。
  兩支 cron 相隔五小時，撞上代表前一輪超時五小時以上，那是事故，該是紅的。
* **手動被擋** → HTTP **409** ＋ `running_job`，**不寫 `job_runs`**。
  那是使用者連點兩次，不是排程事故，寫進去只會污染排程頁。

#### 判讀上線狀態的兩個陷阱

**① 不要從 `backend/config.yaml` 推論排程有沒有開。** 那裡的 `sr_analysis.enabled`
是 repo 預設值（`false`），兩份 compose 也是 `${SR_ANALYSIS_ENABLED:-false}`，
而 **live 由環境變數覆寫**。實際狀態只有查容器才算數——查法與「不要 dump 整份 env」
的理由見 [`development-workflow.md`](./development-workflow.md)。
`candle_gap_detection` 與 `evaluation_universe_sync` 同一個模式。

**② 不要從 `job_runs` 查不到早期輪次就推論「沒跑過」。** 該表 2026-08-25 之前
**只保留當天**（之後才改成 30 天），所以更早的紀錄是被刪掉的，不是沒發生。
要看一支排程實際運作多久，查**它寫入的業務表**——`sr_analysis` 看
`stock_sr_zone_analyses.created_at` 的日期分佈。

兩輪都正常時，每交易日產出 **watchlist 檔數 × 2** 筆分析
（`created_at` 是唯一能區分兩輪的欄位，`analyzed_at` 兩輪相同——它是 K 棒時間）。

⚠️ **不要把當下的檔數寫成常數**，watchlist 會變。要引用實測值就帶 as-of 日期，例如：
**截至 2026-08-31，watchlist 11 檔、每交易日 22 筆、`stock_sr_zone_analyses` 累計 155 次**
（2026-07-14 起算；排程 2026-08-20 上線，之前是零星的人工分析）。

（原記於 `todo.md` T-052 的 review 發現。）

### 研究流程目前是半自動的

`scripts/run-evaluation.sh` 的標的來自 **CLI 參數** `--symbols`，不是從
`evaluation_universe` 讀的；排程版的 `sr_evaluation` job 用的是
`watchlist.Symbols()`。所以「從池撈清單 → 傳給 evaluation」這一步是人工的。

**池是資料保鮮機制，不是研究流程的入口。**

### 哪些研究該用哪一份

這條界線由一個事實決定：**SR scoring 的模型特徵完全不使用籌碼**
（`features.py` 沒有任何 chip 相關欄位）。籌碼只在 decision engine 以解讀層進入，
產生 `chip_interpretation` 與 reasons 文字，不影響 zone 建立與模型預測。

| 研究類型 | 用哪份清單 | 理由 |
|---|---|---|
| Zone 建立與合併參數（T-003 sweep） | **池** | 這一層不碰籌碼，池的 135 檔正是要解決樣本不足 |
| 觸價統計、hold/break calibration、模型 AUC | **池** | 同上 |
| `volatility_profiles` 與 bucket 分層 | **池** | 同上 |
| **decision replay 的 RR / entry-state 分佈** | **watchlist** | 池沒有籌碼，replay 會得到全體 `chip_missing=true` 的偏斜分佈，**與 production 不可比** |

最後一列是容易踩到的：池的規格寫著支撐「evaluation、sweep 與 decision replay」，
但只有前兩者在池上是有效的。這也解釋了 T-003 的順序——先在池上跑 zone 層 sweep，
差異明確後才選少數候選回到 watchlist 跑 decision replay。

### 池要不要加籌碼

**目前不加。** 代價是 135 檔 × 2 requests = 270 requests ÷ 5 req/min
= **每日 54 分鐘**，比日 K 維護（27 分鐘）多一倍；收益只有 decision engine 的一個解讀層，
不改變 zone 品質也不改變模型輸出。要做 decision replay 的分佈研究時，
在 watchlist 上做即可。

選池的產生與維護規格見
[`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)，
表結構見 [`database-schema.md`](./database-schema.md) 的 `evaluation_universe`。

---

## Python 服務

Python 負責回測，獨立運行，透過共用 DB 與 Go 溝通。

```
Go（寫入 backtest_jobs）
    ↓ DB polling（Method A）或 HTTP（Method B）
Python Worker / HTTP Server
    ↓ backtest/engine.py 依 strategy 名稱分派：
    │   - 命中 backtest/modular/strategy.py 的 STRATEGY_PRESETS → 走純 pandas/numpy
    │     的模組化引擎（見 backtest-modular-strategy.md）
    │   - 否則走既有 backtrader 引擎（STRATEGY_MAP）
    ↓ 寫入 backtest_results + backtest_trades（兩條路徑輸出格式一致）
Go API（讀取結果回傳給前端）
```

模組化回測可選擇啟用 `use_chip_filter`，用 `chip_scores.total_score` 與
`chip_min_score` 門檻過濾進場訊號；legacy backtrader 策略收到該選項時會記 warning
並忽略，不中斷回測。

**個股分析**（`analysis` package）跟回測共用「Python 算、Go 存」的分工，但驗證
階段反過來——比對已存的價位跟 candles 大小，不需要重跑 Python 的策略邏輯：

```
Go POST Python /analyze（現況計算：支撐/壓力/進場/停損/停利）
    ↓
Go 寫入 stock_analyses + stock_analysis_levels
    ↓（之後手動觸發，可重複執行）
Go 讀 candles，純 Go 比對支撐/壓力是否突破、停損/停利是否觸及
    ↓
更新 stock_analyses.trade_verification + stock_analysis_levels.status
```

細節見 [stock-analysis.md](./stock-analysis.md)。

**SR Zone Scoring**（同樣是「Python 算、Go 存」，驗證階段跟個股分析一樣是
純 Go，見 sr-zone-scoring.md「十四」）：

```
Go POST Python /sr-zones（zone 建立 + 特徵計算 + 機率模型預測 + 分數推導）
    ↓
Go 寫入 stock_sr_zone_analyses + stock_sr_zones
    ↓（手動 POST /sr-zones/:id/verify，或 daily_close 排程每天自動驗證最近 N 天）
Go 讀 candles，純 Go 比對每個 zone 是否被突破
    ↓
更新 stock_sr_zones.status/broken_at/broken_price
```

**收盤驗證的覆蓋窗口單位是「天」，不是「筆數」**（平日 15:00 的 `sr_zone_verify`，
2026-08-25 起）：

| 設定 | 環境變數 | 預設 | 意義 |
|---|---|---|---|
| `sr_zone_verify.days` | `SR_ZONE_VERIFY_DAYS` | 30 | 往回驗幾天的分析（依 `created_at`） |
| `sr_zone_verify.max_analyses` | `SR_ZONE_VERIFY_MAX_ANALYSES` | 2000 | 單輪處理筆數的硬上限，防止窗口拉長後無限成長。**調得再大也不會超過程式內的 10000** |

**為什麼不是筆數**：舊版寫死「最近 50 筆」，那個數字是分析還沒排程化的年代訂的
（一天 1～3 筆 ≈ 20 個交易日）。分析排程化之後 watchlist 11 檔 × 每日兩輪 = 一天 22 筆，
50 筆只剩約 2.3 個交易日，watchlist 擴到 30 檔就不到一天——**後果不是資料錯誤，
而是更早的分析裡那些 `PENDING` 的 zone 永遠停在 `PENDING`，再也不會被驗到**。
換成天數之後，覆蓋窗口與 watchlist 大小、與每日輪數都脫鉤。

**用 `created_at` 而不是 `analyzed_at`**：後者是日期粒度（同一交易日 17:00 與 22:00
兩輪都寫成當日 00:00，它是「今天這根 K 棒分析過沒」的判定依據），取不出「最近 N 天
實際跑過的分析」。查詢由 `idx_stock_sr_zone_analyses_created_at` 支撐（migration 073）
——既有的 `(symbol, created_at DESC)` leading column 是 symbol，這條不帶 symbol 的
查詢走不到它。

**`max_analyses` 之上還有一道不可突破的 clamp（10000，`scheduler` 的
`maxSRZoneVerifyMaxAnalyses`）**：這支排程沒有 `context.WithTimeout`，`RunDailyClose`
尾端也是無條件呼叫它，那道 clamp 是單輪執行時間唯一的底，不能讓 env 的一個錯字決定。
取 10000 的依據見該常數註解（依實測速率換算約 5 分鐘，且涵蓋 watchlist 150 檔 × 兩輪 × 30 天）。
**清單只取 `id` 與 `symbol`（`SRZoneAnalysisRef`），不撈整份分析**——這是上限在
記憶體上站得住的前提。排程迴圈只用得到這兩欄（ID 傳給 `Verify`、symbol 進失敗 log），
其餘欄位 `Verify` 會自己重查。完整的 `SRZoneAnalysis` 有九個 `RawJSON` 欄位、
實測平均 28 kB／筆，上限 10000 筆時一次載入約 276 MB，在 2GiB host 上會直接 OOM
（2026-08-25 實測：256MB 與 512MB 的 container 都被 kill）。改成 Ref 之後同樣情境
在 **256MB 下跑完**，所以那條查詢不要改回撈整份分析。

**取分析的順序是 `created_at DESC, id DESC`**：`created_at` 只有秒級精度，同一輪的多檔
常落在同一秒；少了 `id` 決勝，撞到上限時邊界那幾筆會在不同引擎之間漂移，某筆分析可能
一直輪不到驗證。**注意單元測試證明不了這件事**——只跑 sqlite，而 sqlite 在同秒時碰巧
就是回 id 遞減（2026-08-25 變異測試實測），這條 `ORDER BY` 的依據是跨引擎確定性的論證，
不是測試保護。

**成本不是限制（2026-08-25 dev postgres 實測）**：672 筆分析、10256 個 zone，
整輪 **20.0 秒**，平均每筆 29.8ms。驗證是本地 DB 往返為主——單筆成本 = 5 次查詢
＋ 每個 zone 一次 `UpdateZoneStatus`，所以成本跟著 **zone 數**走而不只是分析數。
（立案時「45 筆／1 秒」的線性推估給出 660 筆約 15 秒，實測偏高約 33%。）
重跑方式見 `internal/scheduler/sr_zone_verify_devbench_test.go` 的註解。
**watchlist 擴大時要看的是 `job_runs` 裡 `sr_zone_verify` 的 `symbols_total` 有沒有
貼著 `max_analyses`**——貼上去就代表窗口被上限截斷，實際回溯天數已經少於設定值。

訓練是獨立的非同步流程，不在上面這條同步路徑裡：

```
Go POST Python /sr-scoring/train（Go 端路由是 POST /sr-zones/train，立即回 202）
    ↓ 背景 goroutine，Python 端同步執行（可能耗時數十秒到數分鐘）
Python 訓練 hold_model + break_model，寫入 models/*.joblib
```

細節見 [sr-zone-scoring.md](./sr-zone-scoring.md)。

**籌碼分析**由 Go 端同步、計分與查詢，Python 只在模組化回測與 SR Zone v3 模型中讀取
已落地的 `chip_scores`：

```
manual/backfill API 或 daily_close
    ↓
chip.Syncer 從 FinMind 取得三大法人、融資融券、券商分點資料
    ↓
寫入 institutional_trades / margin_trades / broker_trades
    ↓
計算 chip_scores
    ↓
Signal Engine、Backtest、SR Zone Scoring 與前端 Chips 頁面讀取使用
```

`POST /api/v1/chips/sync` 建立 `chip_sync_jobs` 非同步任務；日結同步則使用
`job_runs.job_name=chip_daily_sync`，兩者的進度表不同。

**Position Analysis** 是交易決策快照層，以 immutable ledger 與 AVG projection 重用
SR Zone Scoring：

```
使用者選定 portfolio 後新增 BUY / SELL / ADJUSTMENT transaction
    ↓ 同一 transaction 更新 positions AVG projection
POST /trade-analysis/analyze
    ↓
Go 查同 symbol/timeframe 的 24 小時內 SR 快照；可 force_refresh
    ↓
依 portfolio 內的 FLAT/LONG、固定風險設定、SR Decision 與 zones 計算目標股數
    ↓
寫入 position_analyses 不可變快照
```

Position owner scope 導入後，Position Analysis 以 `portfolio_id + symbol` 作為持倉 scope；
所有 position / trade-analysis API 都必須明確提供 `portfolio_id`，避免不同使用者或群組共用
隱含預設帳本。成本採移動加權平均。
交易事件不可修改或刪除；資料更正必須新增 `ADJUSTMENT`
並記錄原因。`ADJUSTMENT` 是無交易價格、無現金流的 projection 校正，只覆寫校正後
股數與 AVG，不改變由 SELL 累積的 `realized_pnl`；實際成交必須記為 BUY/SELL。
分析輸出包含目前／目標／調整股數、風險金額、RR、觸發與失效條件。`HOLD`
若帶有 `position_action_condition`，必須解讀為條件式持有，並顯示防守線
（`invalidation_price`）、回穩線（`recovery_price`）與 reason codes。若 SR 決策為
Buy/BuySmall、存在有效停損支撐但已無上方壓力，停利目標以可設定的固定 R 倍數推導，
預設為 2R，避免突破後因缺少壓力 zone 而無法建立或增加部位。
每次分析都新增一筆快照，不覆蓋舊結果。

Portfolio 權限由 `PortfolioRepo.CanAccess` 集中判斷：

- `TENANT` portfolio：保留為未來 tenant-level 帳本能力；目前不再建立 shared default portfolio。
- `USER` portfolio：只有 owner user 可讀寫。
- `GROUP` portfolio：group member 可讀；`OWNER` / `ADMIN` 可寫，`MEMBER` / `VIEWER` 不可寫。

Group 角色管理（`GroupRepo.AddMember`）有對應保護：actor 不得改自己的 role；只有 `OWNER` 能授予或
異動 `OWNER`（`ADMIN` 不得碰 `OWNER`）；不得降級最後一名 `OWNER`；被加入者必須**已是** group tenant 的
成員，否則拒絕（不自動補 tenant membership，避免 group admin 把任意 user 拉進 tenant 取得 TENANT
portfolio 寫入權）。`tenant_members.role` 目前不參與授權（`CanAccess` 只看 tenant membership 是否存在），
故所有 tenant membership 一律預設 `MEMBER`；實際讀寫權限由 portfolio owner scope 與 group role 決定。

**Trade Analysis** 是前端與 API 的統一入口。呼叫端提供股票代號與必要的
`portfolio_id`；後端讀取該 portfolio 的 `positions` projection 後自動判斷決策情境：

- `shares > 0`：以 `LONG` 持股情境分析，成本基準使用 AVG。
- 找不到 projection 或 `shares = 0`：以 `FLAT` 空手情境分析，不視為錯誤。

SR Zone 是市場結構層，負責 Data → Features → Score → Evidence；Position
Analysis 是決策快照層，負責把 FLAT/LONG 情境、AVG 成本、風險設定與 SR 結果
收斂成 Action、目標股數、停損、停利、RR、reason/evidence。`/trade-analysis/*`
facade 回傳同一份 `position_analyses` 快照格式，讓新入口不需要先分辨「支撐
壓力分析」或「持股分析」。

Position Analysis 的細節拆解放在 `evidence` JSON：`risk_sizing` 顯示
Risk Budget → Per Share Risk → Max Shares → Excess Shares；`stops` 拆成
Defense Price 與 Structural Stop；`rr` 拆成 Market RR 與 Position RR；`pnl_impact`
顯示執行減碼/出場後的 realized delta 與 unrealized before/after。
`decision_context.mode` 明確標示本次是 `FLAT_ENTRY` 或 `LONG_POSITION`；
`entry_decision` 是空手者的新進場決策，`position_decision` 是已持有者的續抱、
減碼、出場或加碼決策。LONG 情境不得用 `entry_decision` 推論持有建議；FLAT
情境不得用 `position_decision` 推論新進場建議。實際持倉 `position_rr` 只在
Position Engine 使用 AVG 成本與防守線計算，來源標示為 `POSITION_AVG_COST`。

SR 快照由 `SRAnalysisProvider` 統一處理重用與重算。Trade Analysis 與 Position
Analysis 預設會優先重用同 symbol/timeframe 且仍在設定時效內的 SR 快照；呼叫端
指定 force refresh 時才會重新產生。SR Zone 專頁的 `/sr-zones` endpoint 保留「建立
新快照」語意，但可用 `reuse_existing=true` 明確選擇同一套 provider 重用策略。

#### SR 快照刪除與歷史參照政策

`position_analyses.sr_zone_analysis_id` 是 best-effort historical reference，不是
不可刪除的強一致外鍵。Trade Analysis 歷史本身以 `position_analyses` 保存當次的
決策、價格、部位、設定、reason/evidence、trigger/invalidation 等不可變快照；若其
引用的 SR Zone 快照後來被刪除，歷史分析仍可讀取這些已保存欄位，但無法再回查當時
完整的 SR zones 清單。

因此 SR Zone 刪除行為維持目前策略：刪除 `stock_sr_zone_analyses` 與其
`stock_sr_zones`，不因既有 `position_analyses` 引用而阻擋，也不補寫大型 SR snapshot
到 `position_analyses`。需要長期可審計完整 SR zones 的情境，應保留對應 SR 快照或
另行設計專門的歸檔欄位。

#### 已知限制：SR 快照與 Position Analysis 非單一 transaction（刻意不處理）

「建立新 SR 快照」與「建立 position analysis」是**兩次獨立的 DB 寫入**（分屬
`store.SRZoneRepo` 與 `store.PositionRepo`），不是包在同一個跨 repo transaction 裡。
Position transaction 與 projection 本身則必須在同一個 DB transaction 中完成；
`version` 與 row lock 防止併發覆寫。

殘留限制：若在「SR 建立 commit」與「position analysis 建立 commit」之間程序硬崩潰／連線中斷，
會留下一筆沒有任何 `position_analyses` 引用的 SR 快照。**刻意不升級為跨 repo
transaction**，理由：

1. **發生率極低**：僅限兩次 commit 之間毫秒級窗口的硬故障。
2. **無資料損毀、方向安全**：殘留的是一筆合法的 `stock_sr_zone_analyses`，沒有指向缺
   資料的外鍵；失敗的 position analysis 不會產生半套快照。
3. **幾乎無害且會自癒**：SR 快照本就由 SR Zone 頁、排程器、Position 三方共同產生、多數本
   來就無 position analysis 引用，一筆「孤兒」SR 與一筆正常 SR 分析無法區分、價值相同；加上 Position
   分析已有「同 symbol/timeframe 新鮮快照優先重用」機制，該筆若仍新鮮下次會被直接重用。
4. **成本／風險不成比例**：跨 repo transaction 需替兩個 repo 導入共用 executor 抽象並
   改寫 `srZoneRepo.Create`（SR Zone 頁與排程器共用的寫入路徑），回歸風險外溢到持股
   功能之外，不值得為上述殘留窗口投入。

### Nullable 欄位的 JSON 序列化

Go 的 `database/sql.NullFloat64` / `NullString` / `NullTime` 直接拿去
`json.Marshal` 會變成 `{"Float64":123.45,"Valid":true}` 這種內部結構，不是
單純的數字或 `null`，前端拿到後對它做數值運算（例如 `.toFixed()`）會直接
拋型別錯誤——`stock_analyses` 的多個可空欄位（`stop_loss_atr` 等）就中過這個
問題。**任何 API 會回傳的 struct，可空欄位一律用 `internal/store/null.go`
裡的 `store.NullFloat64` / `store.NullString` / `store.NullTime`**（內嵌
`sql.Null*` 保留 `Scan`/`Value` 給 sqlx，另外補上 `MarshalJSON`），不要直接用
標準庫的 `sql.Null*`。

---

## 技術選型

| 元件 | 選擇 | 理由 |
|------|------|------|
| Backend | Go | goroutine 適合多股票並行監控 |
| HTTP Router | Gin | 高效能、中介軟體齊全 |
| Database | SQLite / MySQL / PostgreSQL | 三種環境皆支援，goose 自動 migration |
| Cache | Redis（選填） | 低延遲熱資料；addr 留空則停用。若誤連到 read-only replica，Redis 寫入會短暫降級為 no-op，DB 主流程仍正常 |
| WebSocket | gorilla/websocket | 穩定、廣泛使用 |
| Frontend | Vite + Svelte | 輕量、無 VDOM；build 後 embed 進 Go binary |
| K 線圖 | lightweight-charts | < 50KB、原生 Candlestick |
| Backtest | Python + backtrader，另有純 pandas/numpy 模組化引擎 | 策略研究與驗證；模組化引擎可獨立替換 S/R、進場、停損元件（Strategy Pattern） |
| Auth | JWT（HS256）+ bcrypt | 無狀態 token，密碼安全雜湊 |

---

## 可觀測性：為什麼沒有 metrics 依賴

`backend/go.mod` **沒有 prometheus 或任何 metrics 套件**，整個 backend 也沒有
`/metrics` 端點。趨勢型的觀測改用**一張表加一個查詢端點**：

* 結構化 log 回答「這一次發生了什麼」。
* `sr_identity_stats` 表（一次分析一列）＋ `GET /sr-zones/identity-stats`
  回答「這個月的走勢如何」——例如 alias 命中率是不是在爬。
  設計見 [`database-schema.md`](./database-schema.md)。

**決定性的理由是資料頻率**：這類 metric 一次分析產生一筆，分析排程上線後是
**每個交易日約「watchlist 檔數 × 2」筆**（每日兩輪）——以 2026-08-31 production 的
11 檔估算是每交易日約 22 筆。Prometheus 是為高頻抓取設計的，這個數量級本質上是一張**表**而不是
time series，用它反而要處理「抓取間隔內沒有變化」這種與問題無關的複雜度。

另外兩個理由：引入 metrics 要一併決定 exporter、抓取端、儀表板與保留期，是跨系統決策，
不該夾帶在單一功能裡交付；而這台 host 只有 2GiB，再擺 Prometheus ＋ Grafana 不現實。

**告警之後仍可加**，但要先有抓取端。目前唯一真正需要即時告警的訊號是
`invariant_violations` 非零（不變式被違反，不是「比較差」），而它已經是 Error 級 log。

---

## 批次 vs 串流設計決策

Phase 1 採用**批次計算**：
- FinMind API 為 REST 輪詢，非即時 tick
- 5 分鐘 batch 計算足夠盤中監控需求
- 邏輯簡單、易於測試和驗證

Phase 2 換 Shioaji 後可改為 tick-level streaming rolling sum。

---

## 部署模式

### 開發環境

```
go run ./cmd/server      # SQLite，自動 migration
npm run dev              # Vite dev server（含 API proxy）
python worker.py         # Python worker（選填）
```

### 生產 / Docker

```
docker network create proxy_net 2>/dev/null || true
docker-compose -f docker-compose.postgres.yml -f docker-compose.redis.yml -f docker-compose.yml up --build
```

- PostgreSQL 與 Redis 分別由 `docker-compose.postgres.yml` / `docker-compose.redis.yml` 管理；主 `docker-compose.yml` 負責 backend、python-worker、python-server，並假設 `trading-net` / `proxy_net` 已存在
- Go binary 內嵌前端靜態檔案（單一執行檔）
- Python worker 與 HTTP server 各自獨立 container
