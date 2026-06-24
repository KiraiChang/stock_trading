# API Reference

Base URL：`http://localhost:8080/api/v1`

除 `/auth/*` 外，所有端點皆需在 Header 帶入 JWT Token：

```
Authorization: Bearer <token>
```

---

## Auth API（公開，不需 token）

### POST `/auth/register`

註冊新使用者。**新帳號預設 `status: inactive`，無法立即登入，需由已登入的使用者在管理頁面或透過 `PATCH /users/:id/status` 啟用。**

**Request Body：**
```json
{ "email": "user@example.com", "password": "secret123" }
```

**Response（201）：**
```json
{ "user_id": 1, "email": "user@example.com", "status": "inactive" }
```

### POST `/auth/login`

登入取得 JWT token。

**Request Body：**
```json
{ "email": "user@example.com", "password": "secret123" }
```

**Response（200）：**
```json
{ "token": "eyJhbGci...", "expires_in": 86400 }
```

**錯誤回應：**

| 狀態碼 | 說明 |
|--------|------|
| 401 | 帳號或密碼錯誤 |
| 403 | 帳號尚未啟用（`status != active`），需管理員開通 |

Token 有效期 24 小時。之後請求帶入 `Authorization: Bearer <token>`。

---

## User Management API

### GET `/users`

列出所有使用者（不含密碼雜湊）。

**Response：**
```json
{
  "users": [
    {
      "id": 1,
      "email": "admin@trading.com",
      "status": "active",
      "created_at": "2024-01-15 10:00:00"
    },
    {
      "id": 2,
      "email": "newuser@example.com",
      "status": "inactive",
      "created_at": "2024-01-16 09:30:00"
    }
  ]
}
```

### PATCH `/users/:id/status`

啟用或停用指定使用者。

**Request Body：**
```json
{ "status": "active" }
```

`status` 只接受 `"active"` 或 `"inactive"`。

**Response（200）：**
```json
{ "id": 2, "status": "active" }
```

---

## Candle API

### GET `/candles/:symbol`

取得 K 棒資料。

**Query Parameters：**

| 參數 | 預設 | 說明 |
|------|------|------|
| timeframe | `1d` | `1m`, `5m`, `1d` |
| limit | `60` | 最多 1000 |

**Response：**
```json
{
  "symbol": "2330",
  "timeframe": "1d",
  "candles": [
    {
      "id": 1, "symbol": "2330", "timeframe": "1d",
      "open": 975.0, "high": 980.0, "low": 970.0, "close": 978.0,
      "volume": 25000000, "amount": 24450000000,
      "ts": "2024-01-15T00:00:00+08:00"
    }
  ]
}
```

---

## Indicator API

### GET `/indicators/:symbol`

取得最新技術指標快照。

**Query Parameters：**

| 參數 | 預設 |
|------|------|
| timeframe | `1d` |

**Response：**
```json
{
  "id": 1, "symbol": "2330", "timeframe": "1d",
  "ts": "2024-01-15T00:00:00+08:00",
  "ma5": 975.4, "ma10": 968.2, "ma20": 955.1, "ma60": 920.3,
  "rsi14": 63.5,
  "macd": 8.2, "macd_signal": 6.1, "macd_hist": 2.1,
  "bb_upper": 995.2, "bb_middle": 955.1, "bb_lower": 915.0,
  "atr14": 12.5,
  "vwap": 976.3,
  "vol_ma20": 20000000, "vol_ratio": 1.25
}
```

---

## Signal API

### GET `/signals`

取得訊號記錄。

**Query Parameters：**

| 參數 | 說明 |
|------|------|
| limit | 筆數（預設 50） |
| symbol | 篩選特定股票 |

**Response：**
```json
{
  "signals": [
    {
      "id": 1, "symbol": "2330",
      "signal_type": "BREAKOUT", "direction": "BUY",
      "price": 980.0, "volume": 45000000, "vol_ratio": 2.25,
      "resistance": 975.0, "support": 0.0, "trend": "BULLISH",
      "note": "突破阻力 975.00，量比 2.25x",
      "ts": "2024-01-15T10:30:00+08:00"
    }
  ],
  "total": 1
}
```

---

## Watchlist API

### GET `/watchlist`

取得監控清單。

### POST `/watchlist`

新增股票至監控清單。

**Request Body：**
```json
{ "symbol": "2330", "name": "台積電", "sector": "半導體" }
```

### POST `/watchlist/bulk`

批次新增股票（已存在的 symbol 會更新名稱與產業）。

**Request Body：**
```json
{
  "items": [
    { "symbol": "2330", "name": "台積電", "sector": "半導體" },
    { "symbol": "2454", "name": "聯發科", "sector": "半導體" },
    { "symbol": "2317", "name": "鴻海",   "sector": "電子" }
  ]
}
```

**Response：**
```json
{ "added": 3, "failed": 0, "total": 3 }
```

### DELETE `/watchlist/:symbol`

從監控清單移除股票。

---

## Market API

### POST `/market/backfill`

觸發歷史 K 棒資料補撈（背景執行，立即回傳）。

`symbols` 省略時自動使用 watchlist 全部股票；`days` 預設 120。

**Request Body：**
```json
{ "days": 120, "symbols": ["2330", "2454"] }
```

**Response（202 Accepted）：**
```json
{
  "message": "backfill 已在背景啟動",
  "symbols": 2,
  "days": 120
}
```

> 進度可透過 backend log 觀察；每支股票間隔 200ms（FinMind rate limit）。

---

## Backtest API

### POST `/backtest`

提交回測任務。

**Request Body：**
```json
{
  "strategy": "breakout_v1",
  "symbols": ["2330", "2454"],
  "timeframe": "1d",
  "start_date": "2023-01-01",
  "end_date": "2024-12-31"
}
```

**Response：**
```json
{ "job_id": "bt_20240115_abc123", "status": "pending" }
```

### GET `/backtest`

列出所有回測任務。

### GET `/backtest/:job_id`

取得特定回測任務狀態與結果。

**Response（完成後）：**
```json
{
  "job_id": "bt_20240115_abc123",
  "status": "done",
  "result": {
    "total_return": 0.182,
    "annual_return": 0.091,
    "win_rate": 0.62,
    "max_drawdown": -0.083,
    "sharpe_ratio": 1.42,
    "total_trades": 24,
    "win_trades": 15,
    "loss_trades": 9
  }
}
```

### GET `/backtest/:job_id/trades`

取得回測每筆交易明細。

### DELETE `/backtest/:job_id`

取消或刪除回測任務。

---

## WebSocket

**連線：** `ws://localhost:8080/ws/market`

**訂閱：**
```json
{ "action": "subscribe", "symbols": ["2330", "2454"] }
```

**取消訂閱：**
```json
{ "action": "unsubscribe", "symbols": ["2330"] }
```

**Server 推送事件：**
```json
{ "type": "candle",    "symbol": "2330", "data": { ...Candle } }
{ "type": "indicator", "symbol": "2330", "data": { ...Snapshot } }
{ "type": "signal",    "symbol": "2330", "data": { ...Signal } }
```
