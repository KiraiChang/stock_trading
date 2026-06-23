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

**Dataset：** `TaiwanStockPriceMinute`

```
GET /data?dataset=TaiwanStockPriceMinute&data_id=2330&start_date=2024-01-15&end_date=2024-01-15&token=YOUR_KEY
```

時間戳格式：`2024-01-15 09:01:00`

---

## Rate Limit 處理

- 免費方案：每分鐘約 30 requests
- 系統預設每次請求後等待 200ms（Fetcher.BackfillHistory）
- Scheduler 每 5 分鐘批量更新，一般不會超過限制

---

## Backfill 歷史資料

```go
fetcher.BackfillHistory(ctx, symbols, 90)  // 最近 90 天
```

建議首次啟動時先 backfill 至少 120 天日K，確保所有指標（MA60 需要 60 根）可以計算。
