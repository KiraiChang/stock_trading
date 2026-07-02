# FinMind API 整合說明

## 端點

Base URL：`https://api.finmindtrade.com/api/v4/data`

---

## 日K 資料

**Dataset：** `TaiwanStockPrice`

```
GET /data?dataset=TaiwanStockPrice&data_id=2330&start_date=2024-01-01&end_date=2024-01-31&token=YOUR_KEY
```

**回應欄位對應：**

| FinMind 欄位 | 系統欄位 |
|-------------|---------|
| date | ts（格式 2006-01-02） |
| open | open |
| max | high |
| min | low |
| close | close |
| Trading_Volume | volume |
| Trading_money | amount |

---

## 分K 資料

**Dataset：** `TaiwanStockKBar`（`TaiwanStockPriceMinute` 已下架，v4 API 改用此 dataset）

```
GET /data?dataset=TaiwanStockKBar&data_id=2330&start_date=2024-01-15&token=YOUR_KEY
```

- 單次請求只能拉一天資料（不支援 `end_date` 區間）
- 需要 FinMind **Sponsor 級以上**的 token；帳號等級不足時回傳 400，訊息含
  `"user level"`/`"Sponsor"`，`market.ErrInsufficientTier` 會辨識這種情況並讓
  排程整輪跳過（重試也一定失敗，見 `scheduler.go` 的 `runIntradayJob`）
- 回應欄位：`date`（日期）與 `minute`（`HH:MM:SS`）為分開兩個欄位，需自行組成
  timestamp；**不提供成交金額**，intraday VWAP 目前無法用此 dataset 計算

**帳號等級不足時建議直接關閉排程**，不要依賴上述的「跑下去才發現 400」：
`config.yaml` 的 `finmind.intraday_enabled`（環境變數 `FINMIND_INTRADAY_ENABLED`）
**預設 `false`**，`runIntradayJob` 一開始就會檢查這個設定並直接跳過，不會
每 5 分鐘對 FinMind 發出注定失敗的請求、也不會在 `job_runs` 洗一筆
`skipped` 紀錄。升級到 Sponsor 級以上的帳號後，改成 `true` 即可恢復。

**回應欄位對應：**

| FinMind 欄位 | 系統欄位 |
|-------------|---------|
| date + minute | ts（`YYYY-MM-DD HH:MM:SS`） |
| open/high/low/close | 同名 |
| volume | volume |
| （無） | amount（固定為 0） |

---

## Rate Limit 處理

- 免費方案：每分鐘約 30 requests（依帳號方案而定）
- `market.FinMindClient` 內建 `rateLimiter`，依 `finmind.rate_limit`（`config.yaml`，
  每分鐘請求數）節流，所有請求（日K/分K/backfill）共用同一個節流器
- 對 429（請求過於頻繁）、402（額度用盡）、5xx 做指數退避重試（最多 3 次）；
  400 帳號等級不足則不重試，直接回傳 `ErrInsufficientTier`
- Scheduler 盤中每 5 分鐘批量更新，一般不會超過限制

---

## 與 Fugle 並行

盤中即時行情可選擇並行接入 Fugle MarketData API（`fugle.enabled: true`），
FinMind 仍負責歷史日K/backfill。細節見 [fugle-integration.md](./fugle-integration.md)。

---

## Backfill 歷史資料

```go
fetcher.BackfillHistory(ctx, symbols, 90)  // 最近 90 天
```

建議首次啟動時先 backfill 至少 120 天日K，確保所有指標（MA60 需要 60 根）可以計算。
