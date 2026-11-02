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
6. `scheduler.runIntradayJob` 在 `HasIntradaySource()` 為真時走 `runIntradayBatch`，依 `batch_size`
   分批呼叫 `FetchAndStoreIntradayBatch`。

冪等性由 `candles` 表 `UNIQUE(symbol, timeframe, ts)` 保證，重覆抓取只會 UPSERT 不會重複。

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
* ⚠️ ⛔ 不保證兩輪⛔ 不重疊：批次多、第一次抓取、評估與結束寫入都還要時間（與導入前相同），重試只讓一輪最多晚約 120 秒結束重試的部分。

**某一輪失敗時缺的棒，下一輪成功時就補回來**（回答 I-122「缺的 K 棒會不會補回」）：批次端點一次回傳**當日全部**的 1 分 K，
寫入是 `ON CONFLICT … DO UPDATE`，還在走的那一根也會被下一輪覆蓋。由 `market/intraday_refill_test.go` 走正式的抓取流程
（Yahoo client → `FetchAndStoreIntradayBatch` → SQLite）驗證。⚠️ **13:30 是最後一輪**，那一輪失敗就沒有下一輪可以補
（日 K 由 15:00 的 `daily_close` 另外抓，不受影響）。

**已擱置的後續工作**：批次請求失敗（Yahoo 被限流/封鎖）時的 Yahoo→FinMind fallback
目前未實作，`runIntradayBatch` 在上面的重試之後只記 log 續跑其他批次。這筆與 Fugle→FinMind fallback
一併擱置（見 [todo.md](./todo.md) T-031、T-008）；盤中源相關工作暫不列入近期處理。
