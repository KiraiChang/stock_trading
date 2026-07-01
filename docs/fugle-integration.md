# Fugle API 整合說明

盤中即時行情的並行資料源，與 FinMind 分工：**FinMind 顧歷史/日K/backfill，
Fugle 顧盤中即時**。目前只完成了 REST/WebSocket client 與驗證工具，尚未接上
`Fetcher`/`scheduler` 的自動排程（見文末「尚未完成」）。

---

## 為什麼要接 Fugle

FinMind 盤中分K（`TaiwanStockKBar`）是每 5 分鐘輪詢一次，且需要付費 Sponsor
級 token。免費、不需綁證券商帳戶的 Fugle 公開方案額度：

| 項目 | 額度 |
|------|------|
| WebSocket 連線數 | 1 |
| WebSocket 同時訂閱檔數 | 5 |
| REST 即時行情呼叫頻率 | 60 次/分鐘 |
| REST 歷史行情呼叫頻率 | 60 次/分鐘 |

5 檔的 WebSocket 訂閱數遠低於 watchlist 規模（Phase 1 目標 50~200 檔），無法
全量覆蓋，因此規劃為**兩層式架構**：Tier 1 用 REST 60/min 做全 watchlist 廣度
掃描（取代 FinMind 5 分鐘輪詢），Tier 2 用 WebSocket 僅有的 5 個名額做「熱點」
（接近關鍵價位或有持倉的股票）秒級確認。**Tier 1/Tier 2 的排程整合、熱點名額
管理目前尚未實作**，只有底層的 REST/WS client 完成。

---

## REST API

Base URL：`https://api.fugle.tw/marketdata/v1.0/stock`

認證：Header `X-API-KEY: <API_KEY>`

### 即時報價

```
GET /intraday/quote/{symbol}
```

### 當日 1 分K

```
GET /intraday/candles/{symbol}?timeframe=1
```

- `timeframe` 可選 `1`/`3`/`5`/`10`/`15`/`30`/`60`（分K）
- 不支援指定日期區間，只回傳當日資料
- 回應：`{"date","symbol","data":[{"date","open","high","low","close","volume","average"}, ...]}`，
  `data[].date` 為 RFC3339 格式

### 錯誤碼

| 狀態碼 | 說明 |
|--------|------|
| 401 | API Key 無法驗證 |
| 403 | 目前方案不支援此功能 |
| 404 | 商品代碼不存在 |
| 429 | 呼叫次數已達上限 |

`FugleQuoteClient.doGet`（`backend/internal/market/fugle_quote.go`）對非 200
回應會嘗試解析 `{"message": "..."}`，解析失敗則回傳截斷後的原始 body。

---

## WebSocket API

連線端點：`wss://api.fugle.tw/marketdata/v1.0/stock/streaming`

### 協定（已依官方文件實測確認）

```
連線後 → 送 auth
{"event":"auth","data":{"apikey":"<API_KEY>"}}
← {"event":"authenticated","data":{"message":"..."}}  或 {"event":"error",...}

訂閱 → 
{"event":"subscribe","data":{"channel":"candles","symbol":"2330"}}
← {"event":"subscribed","data":{"id":"<CHANNEL_ID>","channel":"candles","symbol":"2330"}}

取消訂閱 →
{"event":"unsubscribe","data":{"id":"<CHANNEL_ID>"}}
← {"event":"unsubscribed","data":{"id":"<CHANNEL_ID>"}}

伺服器每 30 秒主動送：{"event":"heartbeat","data":{"time":<timestamp>}}
```

可訂閱的 channel：`trades`（成交）、`candles`（分鐘K）、`books`（五檔）、
`aggregates`（聚合行情）、`indices`（指數）。目前只用到 `candles`。

### 訂閱 candles 後的推送（實測結果）

訂閱成功後，**第一筆推送是 `"event":"snapshot"`**，把當天至今的整包 1 分K
一次送完（結構跟 REST `/intraday/candles` 相同，但 `id`/`channel` 是跟
`event`/`data` **同層**的欄位，不是包在 `data` 裡面——這跟 `subscribed` 事件
的欄位位置不一致，是實測才發現的細節）：

```json
{
  "event": "snapshot",
  "data": { "date": "...", "symbol": "2330", "timeframe": "1", "data": [ {...一根根K棒...} ] },
  "id": "<CHANNEL_ID>",
  "channel": "candles"
}
```

`FugleStreamClient.handleSnapshot`（`backend/internal/market/fugle_stream.go`）
已依此格式解析，逐根回呼 `onCandle`。

**尚未確認**：盤中持續的即時更新（每根分K收線前的推播）長什麼樣子——因為
第一次實測是在收盤後做的，只觀察到 `snapshot` + `heartbeat`，沒有看到新成交
觸發的推播。`fugle_model.go` 的 `fugleCandleData`（單根K棒格式）是依 REST
格式與業界慣例**推測**的欄位，尚未經過盤中實測驗證。

**下一步驗證方式**：於台股盤中時段（09:00–13:30）執行：

```bash
cd backend
$env:FUGLE_ENABLED="true"; $env:FUGLE_API_KEY="<key>"
go run ./cmd/fugle-check -symbol 2330 -duration 120s
```

觀察 `raw:` 開頭的原始訊息，確認即時更新的 `event` 名稱與欄位，並回頭校正
`fugle_stream.go`/`fugle_model.go`。

---

## 程式碼結構

| 檔案 | 職責 |
|------|------|
| `backend/internal/market/source.go` | `MarketDataSource`/`QuoteSource`/`StreamingSource` 介面 |
| `backend/internal/market/fugle_model.go` | REST 回應與 WS 事件的資料結構 |
| `backend/internal/market/fugle_quote.go` | REST client（`QuoteSource` 實作），60/min rate limiter |
| `backend/internal/market/fugle_stream.go` | WebSocket client（`StreamingSource` 實作），含斷線重連、心跳偵測 |
| `backend/internal/market/fetcher.go` | `SetFugle`/`FetchAndStoreFugleIntraday`/`SubscribeRealtime` 掛載點 |
| `backend/cmd/fugle-check/main.go` | 獨立驗證工具，不掛載到主服務 |

---

## 設定

`backend/config.yaml` 的 `fugle:` 區塊（對應環境變數 `FUGLE_*`）：

```yaml
fugle:
  api_key: "YOUR_FUGLE_API_KEY"        # FUGLE_API_KEY，正式金鑰請用環境變數注入，不要寫死在 config.yaml
  rest_base_url: "https://api.fugle.tw/marketdata/v1.0/stock"
  ws_endpoint: "wss://api.fugle.tw/marketdata/v1.0/stock/streaming"
  enabled: false                       # FUGLE_ENABLED，驗證通過前預設關閉
  quote_rate_limit: 60                 # FUGLE_QUOTE_RATE_LIMIT
  max_subscriptions: 5                 # FUGLE_MAX_SUBSCRIPTIONS
  reconnect_max_sec: 60                # FUGLE_RECONNECT_MAX_SEC
```

`docker-compose.yml`/`deploy.sh` 已對應加上 `FUGLE_ENABLED`/`FUGLE_API_KEY`
環境變數注入點。`cfg.Fugle.Enabled=false`（預設）時，`cmd/server/main.go`
完全不會組裝 Fugle client，行為與導入前一致。

---

## 尚未完成（Roadmap）

1. **盤中即時更新格式驗證**（見上方「尚未確認」段落），這是後續工作的前提。
2. **Tier 1 廣度掃描**：REST round-robin 排程，取代 `scheduler.go` 的
   `runIntradayJob` 中對 FinMind 分K 的呼叫。
3. **Tier 2 熱點名額管理**：依「接近壓力/支撐/爆量臨界」或「是否有持倉」動態
   決定哪 5 檔股票佔用 WebSocket 訂閱名額。
4. **Fugle 異常時 fallback 回 FinMind**：`market.intraday_source`/
   `fallback_to_finmind` 設定尚未實作對應邏輯。
