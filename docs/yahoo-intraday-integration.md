# Yahoo 股市盤中資料源

盤中 1 分K 的並行資料源。與 FinMind／Fugle 的分工定位：**FinMind 顧歷史/日K/backfill，
盤中即時由 Fugle 或 Yahoo 提供**。

**現況**：Yahoo 已實作為 Tier-1 批次盤中源，落地於 `backend/internal/market/yahoo_quote.go`
／`yahoo_model.go`（實作 `BatchQuoteSource`）、`backend/cmd/yahoo-check`（實盤驗證工具），
設定為 `config.YahooConfig`（`base_url`/`enabled`/`rate_limit`/`batch_size`，預設 `enabled: false`）。
掛載後 `scheduler.runIntradayJob` 的盤中 job 會優先走 Yahoo 批次路徑（`runIntradayBatch`，免 token），
未掛載時退回 FinMind 分K。**目前決策**：盤中源相關工作暫不列入近期處理，
先沿用既有資料流程；Yahoo→FinMind fallback（[todo.md](./todo.md) T-031）與
實盤時段覆蓋率/延遲/封鎖風險驗證（T-032）已改為擱置，等後續有更合適的盤中
資料源或明確需求時再重新評估。

---

## 為什麼評估 Yahoo

現況：FinMind 盤中分K（`TaiwanStockKBar`）需付費 Sponsor 級 token 且預設關閉；Fugle 的
REST/WebSocket client 已實作但尚未接上排程器（見 [fugle-integration.md](./fugle-integration.md)
文末 Roadmap、todo 的 T-006~T-008）。也就是說**目前沒有任何實際掛上排程、免費可用的盤中分K來源**。

Yahoo 股市網頁前端內部使用的 `FinanceChartService.ApacLibraCharts` 端點實測**免 token、免費、
且單次請求可批次帶多檔**，對全市場掃描（Phase 2 ~1900 檔）有 FinMind/Fugle 所沒有的批次優勢，
因此評估作為**新的 Tier-1 盤中源**，與 Fugle Tier-1 並列/擇一。

---

## API 端點

```
GET https://tw.stock.yahoo.com/_td-stock/api/resource/FinanceChartService.ApacLibraCharts;autoRefresh=<ts>;symbols=["2330.TW","0050.TW"];type=tick
```

- 這是 **matrix 參數**（以 `;` 分隔）而非傳統 query string：`autoRefresh` / `symbols` / `type` 直接接在 path 後。
- `symbols` 為 JSON 陣列，`[` `]` `"` 需 URL-encode（`%5B` `%22` `%5D`）；用 `curl` 測試時要加 `-g` 停用 glob，否則 `[]` 被當範圍語法而失敗。
- `autoRefresh` 是前端輪詢用的時間戳（cache-buster），伺服器不強制驗證。
- `type=tick` **名稱有誤導性**：回傳的不是逐筆 tick，而是 1 分鐘 OHLCV K 棒 + 即時快照（見下節）。
- 無需任何 API key／token；實測 HTTP 200。

---

## 回傳結構（實測）

回傳為 JSON **陣列**，每檔一個 element：`[{ "symbol": "...", "chart": {...} }, ...]`。
每個 `chart` 內含：

### `chart.timestamp[]` + `chart.indicators.quote[0]` — 1 分K OHLCV（核心）

| 欄位 | 說明 |
|------|------|
| `chart.timestamp[]` | Unix 秒陣列，實測 270 筆、間隔固定 60 秒，對應台股 09:00–13:30 完整交易日（`chart.meta.gmtoffset=28800`，即 UTC+8） |
| `chart.indicators.quote[0].open/high/low/close` | 與 timestamp 對齊的 OHLC 陣列 |
| `chart.indicators.quote[0].volume` | 與 timestamp 對齊的每分鐘成交量 |

→ 標準 1 分K OHLCV，可無損對映到 `market.Candle`（`Timeframe:"1m"`）與 `candles` 表，**不需改 DB schema**。

### `chart.quote` — 即時快照（加值欄位）

`price / bid / ask / openPrice / previousClose / dayHighPrice / dayLowPrice / change /
changePercent / volume / previousVolume / turnoverM / avgPrice（≈VWAP）/ limitUp / limitDown /
limitUpPrice / limitDownPrice / marketStatus / refreshedTs`。

其中 `bid/ask`、`avgPrice`（VWAP proxy）、`turnoverM`（成交金額，百萬）是 FinMind 分K 沒有的加值欄位；
`refreshedTs` 可用於延遲驗證，`marketStatus`（如 `close`）可判斷盤中/盤後。

### `chart.meta` — 商品中繼資料

`name / quoteType（EQUITY|ETF）/ scale / priceHint / regularMarketPrice / previousClose /
limitUpPrice / limitDownPrice / regularMarketTime / timezone / currentTradingPeriod.regular{start,end}`。

`priceHint=2` 對應顯示小數位數，與 `candles` 表 `DECIMAL(10,2)` 相容，數值已是 TWD、無需再乘 scale。

### 原始樣本（節錄）

```json
[
  {
    "symbol": "2330.TW",
    "chart": {
      "meta": { "name": "台積電", "quoteType": "EQUITY", "gmtoffset": 28800,
                "previousClose": 2420, "limitUpPrice": 2660, "limitDownPrice": 2180,
                "currentTradingPeriod": { "regular": { "start": 1784077200, "end": 1784093400 } } },
      "timestamp": [1784077200, 1784077260, ...],            // 270 筆，間隔 60s
      "indicators": { "quote": [ { "open":[null,2425,...], "high":[null,2430,...],
                                   "low":[null,2420,...], "close":[null,2425,...],
                                   "volume":[null,2262,502,...] } ] },
      "quote": { "price":"2440", "bid":"2435", "ask":"2440", "dayHighPrice":"2460",
                 "dayLowPrice":"2415", "volume":"28488000", "turnoverM":"69577.95",
                 "avgPrice":"2442", "marketStatus":"close", "refreshedTs":"2026-07-15T06:30:00Z" }
    }
  }
]
```

---

## 欄位對映（Yahoo → `market.Candle`）

| `market.Candle` | Yahoo 來源 |
|---|---|
| `Symbol` | element 的 `symbol`（如 `2330.TW`；寫入前視需要去尾碼） |
| `Timeframe` | 固定 `"1m"` |
| `Timestamp` | `chart.timestamp[i]`（`time.Unix(ts, 0)`） |
| `Open/High/Low/Close` | `chart.indicators.quote[0].{open,high,low,close}[i]` |
| `Volume` | `chart.indicators.quote[0].volume[i]` |
| `Amount` | 無逐棒金額；只有整日 `quote.turnoverM`。逐棒 Amount 留空或不填 |

解析時**須跳過任一 OHLC 為 `null` 的棒**（比照 `FugleQuoteClient` 對 parse 失敗 `continue`）。

---

## 與 FinMind / Fugle 的定位差異

| 項目 | FinMind 分K | Fugle Tier-1 REST | Yahoo |
|------|------------|-------------------|-------|
| 費用 / 認證 | 付費 Sponsor token | 免費，需 API key | 免費，無需 token |
| 批次多檔 | 否（一檔一請求） | 否（一檔一請求） | **是（單次帶多檔）** |
| 逐筆 tick | 否 | 否 | **否**（名為 tick 實為分K） |
| 加值欄位 | — | quote | bid/ask、avgPrice、turnover |
| 穩定性 | 官方 API | 官方 API | **非官方，無 SLA** |

**Yahoo 不滿足 Shioaji tick-level 的 roadmap**（todo 的 T-009 / T-012 之 tick-level volume profile 仍需
Shioaji）；它等價/替代的是 FinMind 分K 與 Fugle Tier-1 的**廣度掃描**角色。

---

## 盤後聚合成日 K 的可行性評估（2026-08-11 實測）

評估「用這個端點在盤後批次收全台股資料」是否可行。結論：**價格可用且與 FinMind 完全一致，
成交量不可直接使用**。

### 這個端點取不到歷史日 K

`chart.meta.validRanges` 宣告支援 `1d/5d/1mo/…/max`，但**實際傳什麼都沒有用**：

| 傳法 | 結果 |
|---|---|
| matrix `;interval=1d;range=1mo` | HTTP 400 |
| query `?range=1mo&interval=1d` | 200，但仍回 `dataGranularity=1m, range=1d` |
| matrix `;type=day` / `daily` / `history` … | 200，granularity 不變（`type` 只被白名單接受，不影響輸出） |

所以它是**當日分K 專用**端點，不能取代 FinMind 抓歷史。唯一可行的用法是
**盤後把當日 1 分K 聚合成一根日 K**（open=首根、high=最大、low=最小、close=末根、volume=加總）。

### 價格：10 檔全數完全一致

2026-08-11 收盤後，對 2330／2454／0050／2317／1101／2891／2308／1216／2002／2603
做「Yahoo 分K 聚合」對「FinMind `TaiwanStockPrice` 日 K」的比對，
**O/H/L/C 最大差異為 `0.0000`**。

### 成交量：單位是「張」，換算後仍系統性短少

Yahoo 的 `volume` 是**張**（lot），FinMind 的 `Trading_Volume` 是**股**。乘 1000 換算後仍偏低，
且幅度隨標的差異很大：

| 短少幅度 | 標的 |
|---|---|
| −0.3% ~ −0.5% | 1101、2002、1216 |
| −5.9% | 2330 |
| −7.8% | 2454 |

**收盤集合競價有被收到**——2330 的 13:30 那根是 2662 張、為全日最大，所以短少不是漏掉收盤。
可能來源是零股交易、盤後定價交易，或每分鐘取整的累積誤差；**成因未確定**。

> **實務結論：量能類判斷（volume spike、量比、VWAP）不可使用這個來源。**
> 5~8% 的偏差足以讓門檻型判斷失真，而且偏差幅度與標的相關，不是可以用固定係數修正的。

### 批次效率很高，但一檔壞代號會拖垮整批

| 項目 | 實測 |
|---|---|
| 批次上限 | 至少 **50 檔**（10／20／40／50 皆完整回傳） |
| 耗時 | **幾乎與檔數無關**（10 檔 4.6s、50 檔 5.2s） |
| 上櫃 `.TWO` | ✅ 橘子、穩懋、元太皆正常 |
| **混入不存在的代號** | ❌ **整批 HTTP 404** |

最後一項對全市場抓取最關鍵：批次前**必須**用 `stock_symbols.is_listed` 過濾，
且要有整批失敗時退成小批／逐檔的機制，否則無法定位是哪一檔造成失敗。

批次效率的量級差異：1,900 檔約 38 批，在 `rate_limit=20/分` 下約 **2 分鐘**；
FinMind 逐檔在同樣速率要 **95 分鐘**。

### 尚未驗證

- 只測了**單一交易日**、10 檔樣本，跨日穩定性未知。
- 低流動性／當日無成交的冷門股行為未測（先前已知盤後 ETF 的 minute 陣列可能全為 null，
  見下方「風險與限制」）。
- 成交量短少的確切成因。

---

## 風險與限制

- **非官方 API**：無 SLA、格式可能無預警變動、可能有反爬/封鎖、無文件化 rate limit。→ 已保守節流
  （`rate_limit` 預設 20/分、批次請求計為一次），比照 Fugle 做「先驗證再上線」（獨立 `cmd/yahoo-check`
  工具；實盤時段驗證見 [todo.md](./todo.md) T-032）。
- **陣列覆蓋率不一致**：實測盤後 `0050.TW`（ETF）的 minute 陣列全為 `null`（但快照正常），
  `2330.TW` 正常；首筆常為 `null`（盤前分鐘）。需在**實盤時段**驗證覆蓋率，確認 null 僅盤後/盤前出現、
  而非特定商品類型（ETF）系統性缺值。
- **合規性**：屬網頁前端內部端點，非公開文件化服務，使用需自行評估 Yahoo 服務條款。

---

## 實作方式（現況）

沿用現有 Fugle `QuoteSource` 模式（`source.go` 介面 → client 實作 → `Fetcher` 掛載 → `scheduler`
呼叫），並擴充批次能力發揮 Yahoo 的多檔優勢。已落地的結構：

1. `source.go` 的 `BatchQuoteSource` 介面（`FetchIntradayCandlesBatch` + `RateLimit` + `BatchSize`）。
2. `yahoo_quote.go` + `yahoo_model.go` 實作，重用 `finmind.go` 的 `newRateLimiter` / `truncateBody`；
   解析時跳過任一 OHLC 為 `null` 的棒，timestamp 統一轉台北時區。
3. `config.YahooConfig`（`base_url/enabled/rate_limit/batch_size`），預設 `enabled: false`。
4. `Fetcher` 掛載點一般化（`SetIntradaySource` + `FetchAndStoreIntradayBatch`，另有 `HasIntradaySource`
   / `IntradayBatchSize` 供排程判斷），與 Fugle（`SetFugle`）並存。
5. `cmd/yahoo-check` 獨立驗證工具（不掛主服務，印原始 payload 與解析後 candles）。
6. `scheduler.runIntradayJob` 在 `HasIntradaySource()` 為真時走批次路徑，依 `batch_size`
   分批呼叫 `FetchAndStoreIntradayBatch`（回傳逐檔結果，見下方「逐檔結果、能不能評估與存證」）。

冪等性由 `candles` 表 `UNIQUE(symbol, timeframe, ts)` 保證，重覆抓取只會 UPSERT 不會重複。

### 逐檔結果、能不能評估與存證（T-088＋I-103）

2026-11 起。原則（使用者 2026-11-20）：⛔ **不全面依靠第三方資訊決定系統是否正常**——「Yahoo 有回應」⛔ 不等於「資料可用」，
「`job_runs` success」⛔ 不等於「每一檔都完成評估」；並保留第三方資料異常時的矯正手段。

* **逐檔結果**（`market.IntradaySymbolResult`）：`FetchAndStoreIntradayBatch` 的 error 只代表整批請求失敗；請求成功時依請求的順序回傳每一檔：
  `written`（帶最後一根的時間與根數）、`no_data`（回應裡沒有這檔或全是 null 棒，⛔ 不是失敗）、`store_failed`（寫入失敗）；
  整批失敗時那一批每一檔都是 `fetch_failed`。FinMind 的逐檔路徑（`FetchAndStoreMinute`，live 沒有啟用）同一套結果，**空回應是 `no_data`**。
* **重試的逐檔結果**：層二重試時 HTTP 成功，逐檔照實記（例：A 寫入成功、B 寫入失敗、C 沒有資料，三檔分別記）；
  每一檔另記這一批的嘗試次數與 `http_recovered`，「HTTP 救回」⛔ 不等於「寫入成功」。重試預算到期時依實際停在哪個階段分類：
  已提交 `written`、HTTP 200 已拿到但寫入階段逾時 `store_failed`（`retry_budget`）、確認沒有 `no_data`、回應本身沒拿到 `fetch_failed`（`retry_budget`）。
  層二的 log 訊息與觸發條件⛔ 不變（I-123 的觀察工具在用），只新增欄位。
* **評估之前先判定**（每一檔）：

  | 這一輪的資料 | 評估 |
  |---|---|
  | `written`，**輸入指紋**跟上一次成功評估不同（或沒有成功評估的紀錄） | ✅ 評估 |
  | `written`，指紋跟上一次成功評估相同 | ⛔ 不評估（`unchanged`：引擎讀到的 K 棒完全相同） |
  | `no_data`、`fetch_failed`、`store_failed` | ⛔ 不評估——⛔ 不從舊 K 棒產生或推送訊號，也⛔ 不被當成「未觸發」 |

  * **輸入指紋**：寫入之後從 DB 讀最近 `indicator.Lookback`（120）根 1 分 K（涵蓋訊號引擎的 100 根），依時間排序，對每一根的時間與 OHLCV 取 SHA-256。
    同時間戳的棒被更新、較早的棒被補回或修訂，指紋都會變。
  * 「上一次成功評估」＝最近一筆評估成功（`no_signal`／`signal_sent`／`signal_suppressed`／`withheld_stale`／`degraded`）的紀錄；
    評估失敗、手動補抓、中斷都⛔ 不算，下一輪會再評估。
  * **延遲**（首版門檻 10 分鐘，`scheduler.IntradayStaleAfter`）：最後一根落後這一輪的時刻超過門檻時，照常計算，但觸發時⛔ 不當成即時訊號送出
    （訊號引擎的 `Withhold`：⛔ 不 reserve、⛔ 不寫 `signals`、⛔ 不推播），結果記 `withheld_stale`；資料追上之後條件仍成立就正常送出。
    延遲跟抓取、寫入、評估結果分開記，狀態 API 依當下時間算，盤中停止更新時也看得到。
  * 每一檔的評估結果：`no_signal`、`signal_sent`、`signal_suppressed`（觸發但被判重擋下）、`withheld_stale`、`degraded`、`evaluate_failed`、
    `not_evaluated`（原因 `fetch_failed`／`store_failed`／`no_data`／`unchanged`）。「未觸發」只對應 `no_signal`。
* **鎖與上限**：盤中一輪與手動補抓共用一把鎖、依序執行（兩輪排程之間也是），較舊的請求⛔ 不會晚回來覆寫較新的 K 棒。
  **先拿鎖再建 `job_runs`**，起訖就是實際執行區間。盤中等鎖最多 60 秒，逾時⛔ 不執行：`job_runs` 記 `skipped`＋`lock_wait_timeout`、
  `intraday_rounds.outcome=skipped_lock_timeout`、⛔ 不寫逐檔的列。手動補抓等鎖最多 30 秒（逾時回 409）；**整段持鎖最多 45 秒**（小於盤中等鎖的上限）：
  期限從拿到鎖就開始算（建 run、讀既有的棒、抓取與寫 K 棒都在期限內），K 棒寫完就放鎖，存證與 `job_runs` 的收尾在鎖外，收尾卡住⛔ 不會讓下一輪等鎖逾時。
* **讀不到自選清單**：這一輪明確記成 `failed`（摘要 `watchlist_fetch:…`；`intraday_rounds.outcome=failed`），⛔ 不是用 total=0、failed=0 推出來的 success。
* **存證**：每一輪、每一檔寫進 `intraday_rounds`／`intraday_symbol_runs`（`database-schema.md`），行情段在抓取與寫入之後、評估段在那一檔評估完成之後；
  ⛔ 不放 Redis（Redis 停用時會跟著消失）。存證寫失敗時照跑、照規則評估，`job_runs` 摘要加 `evidence_failed:N`、該輪至少 `partial`；
  失敗的方向是「多評估」（查不到上一次成功評估就再評估，同一個訊號由判重擋下）。建立 run 失敗時照跑，只是沒有逐輪紀錄（log `intraday evidence unavailable`）。
* **畫面**：自選清單每一檔下方一行，行情與評估分兩段（`GET /intraday/status`；Dashboard 盤中每 60 秒、非盤中每 5 分鐘重抓，開盤前開著的頁面開盤後會自動恢復 60 秒）。
  手動補抓比排程新時依補抓的逐檔結果顯示：寫入成功「已補抓」、沒有當日 K 棒「補抓：沒有當日 K 棒」、失敗「補抓失敗（原因）」，都保留最後有效 K 棒。最近一輪比某一檔的逐檔列新時以輪次紀錄為準
  （等鎖逾時「本輪未執行」、讀不到自選清單「本輪執行失敗」、還在跑「本輪執行中」、已結束卻沒有這一檔的列「本輪紀錄不完整」，都顯示本輪未評估並保留上一輪的最後有效 K 棒與評估時間）；評估段還沒寫時分「本輪評估中」與「本輪評估未完成（中斷）」。
  原因只顯示原因碼對照的中文，⛔ 不顯示原始錯誤。
* **手動補抓**（`POST /scheduler/intraday-refill/run`，自選清單標題列的「補抓當日分 K」）：只限當日、自選清單；**只寫 K 棒、⛔ 不評估訊號**，
  收盤後也可以執行（13:30 最後一輪失敗時用）；⛔ 不推進上一次成功評估（下一輪指紋不同就照常評估）。回報每一檔新增／更新／不變的根數、
  當日的最後一根與根數（寫入進度，⛔ 不等於已補齊），以及**仍無法確認的缺口**：交易時段內（09:00～13:25 與 13:30；13:26～13:29 是收盤集合競價，⛔ 不列）
  DB 沒有 K 棒的分鐘，可能是當下無成交，也可能是資料缺漏，⛔ 不判定。結果記 `job_runs`（`intraday_refill`，⛔ 不在 `/scheduler/status` 的排程清單裡）。
* **還沒做的矯正手段**：空窗期間的「事後補判」（引擎只看最新一根，補判要另外設計）、備援資料源（T-031、T-008）。

### 批次失敗的同一輪重試

2026-11-02 起（原記於 `issue.md` I-123～I-126；`job_retry.enabled` 開啟時才生效，**預設關閉**）。
I-122 在 live 看到 31 輪批次失敗，多半是 DNS 逾時這類暫時性錯誤：

* 批次抓取失敗、原因是暫時性（`market.IsTransient`，規則見 [`data-pipeline.md`](./architecture/data-pipeline.md)「什麼算暫時性」）
  → 等 10 秒重試，再失敗等 20 秒再試一次（每批最多 2 次）；仍失敗才計入失敗數，接著照原本的流程評估。
  4xx（含 429 限流）、解析錯誤⛔ 不重試。
* **整輪的重試預算 120 秒**：每一輪開始時建一個「開始時刻 ＋ 120 秒」到期的重試 context，重試的等待、Yahoo 節流器的等待、
  HTTP 請求與寫 K 棒**都用它**，到期就中止（不論有幾批）。每一批的**第一次**抓取⛔ 不受它限制。
* **重試成功**要同時滿足：`FetchAndStoreIntradayBatch` 回傳 `nil`，**而且**返回時重試 context 還沒到期——
  它遇到逐檔寫入失敗只記 log、最後仍回 `nil`，只看回傳值會把「HTTP 成功、寫入途中預算到期」誤當成成功。
  其他原因的逐檔寫入失敗仍給不出逐檔資訊（`issue.md` I-103 的範圍）。
* 層二的 log（重試成功、重試失敗、預算用完、最後的 `intraday batch fetch failed`）都帶 `run_id`；`first_cause` 是第一次失敗的原因碼、`cause` 是當次的原因碼。原因碼由 `market.TransientCause` 決定：封閉值域、只看型別（DNS 逾時、網路不可達、連線被拒、5xx …），⛔ 不含錯誤原文；`IsTransient` 是同一個判定的另一個輸出，兩者⛔ 不會分歧（2026-11-04 R4；2026-11-12 隨主機搬遷上 live）。
* ⚠️ ⛔ 不保證兩輪⛔ 不重疊：批次多、第一次抓取、評估與結束寫入都還要時間（與導入前相同），重試只讓一輪最多晚約 120 秒結束重試的部分。

**某一輪失敗時缺的棒，下一輪成功時就補回來**（回答 I-122「缺的 K 棒會不會補回」）：批次端點一次回傳**當日全部**的 1 分 K，
寫入是 `ON CONFLICT … DO UPDATE`，還在走的那一根也會被下一輪覆蓋。由 `market/intraday_refill_test.go` 走正式的抓取流程
（Yahoo client → `FetchAndStoreIntradayBatch` → SQLite）驗證。⚠️ **13:30 是最後一輪**，那一輪失敗就沒有下一輪可以補
（日 K 由 15:00 的 `daily_close` 另外抓，不受影響）；要補當日分 K 用手動補抓（上方「逐檔結果、能不能評估與存證」）。

**已擱置的後續工作**：批次請求失敗（Yahoo 被限流/封鎖）時的 Yahoo→FinMind fallback
目前未實作，`runIntradayBatch` 在上面的重試之後只記 log 續跑其他批次。這筆與 Fugle→FinMind fallback
一併擱置（見 [todo.md](./todo.md) T-031、T-008）；盤中源相關工作暫不列入近期處理。
