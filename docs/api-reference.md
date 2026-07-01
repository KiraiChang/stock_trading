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

> 進度可透過 backend log 觀察；請求頻率依 `finmind.rate_limit`（每分鐘請求數，`config.yaml`）節流，非固定間隔。前端「歷史資料回補」頁面（`/backfill`）提供勾選監控清單股票的介面。

---

## Backtest API

回測是**非同步 job 模式**：`POST` 送出後立即回傳 `pending` 狀態的 job，實際計算
由 Python worker（輪詢）或 HTTP server（即時推播）在背景執行，需要輪詢
`GET /backtest/:job_id` 直到 `status` 變成 `done`/`failed` 才會有 `result`。
前端「策略回測」頁面（`/backtest`）已內建每 5 秒輪詢的邏輯。

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

`strategy` 可用值：

| 值 | 引擎 | 說明 |
|----|------|------|
| `breakout_v1` | backtrader | 與 Go signal engine 1:1 對齊的既有策略 |
| `breakout_swing_atr_v1` | 模組化（純 pandas/numpy） | Swing High/Low 支撐壓力 + 突破進場 + ATR 停損 |
| `breakout_volprofile_composite_v1` | 模組化 | Volume Profile 支撐壓力 + 突破進場 + 複合停損 |
| `pullback_atrchannel_structural_v1` | 模組化 | ATR 通道支撐壓力 + 回測支撐進場 + 結構停損 |
| `pullback_swing_composite_v1` | 模組化 | Swing High/Low 支撐壓力 + 回測支撐進場 + 複合停損 |

模組化策略的完整數學定義見 [backtest-modular-strategy.md](./backtest-modular-strategy.md)。

**Response（201 Created）：**
```json
{
  "job": {
    "job_id": "bt_20260115_103000_000",
    "type": "backtest",
    "strategy": "breakout_swing_atr_v1",
    "symbols": "[\"2330\",\"2454\"]",
    "timeframe": "1d",
    "start_date": "2023-01-01",
    "end_date": "2024-12-31",
    "status": "pending",
    "trigger": "manual",
    "created_at": "2026-01-15T10:30:00+08:00"
  }
}
```

### GET `/backtest`

列出所有回測任務（依 `created_at` 由新到舊）。

**Query Parameters：** `limit`（預設 20，最多 200）

**Response：**
```json
{ "jobs": [ { "job_id": "...", "status": "done", "...": "..." } ], "total": 1 }
```

### GET `/backtest/:job_id`

取得特定回測任務狀態與結果；`result` 在任務未完成時為 `null`。

**Response（完成後）：**
```json
{
  "job": { "job_id": "bt_20260115_103000_000", "status": "done", "...": "..." },
  "result": {
    "job_id": "bt_20260115_103000_000",
    "strategy": "breakout_swing_atr_v1",
    "total_return": 0.182,
    "annual_return": 0.091,
    "win_rate": 0.62,
    "max_drawdown": 0.083,
    "sharpe_ratio": 1.42,
    "total_trades": 24,
    "win_trades": 15,
    "loss_trades": 9,
    "avg_pnl": 3250.5
  }
}
```

### GET `/backtest/:job_id/trades`

取得回測每筆交易明細。

**Response：**
```json
{
  "job_id": "bt_20260115_103000_000",
  "trades": [
    {
      "symbol": "2330", "direction": "BUY",
      "entry_time": "2023-03-01T00:00:00+08:00", "exit_time": "2023-03-10T00:00:00+08:00",
      "entry_price": 550.0, "exit_price": 570.0,
      "size": 1818.18, "pnl": 34500.0, "pnl_pct": 0.0345, "commission": 1560.2
    }
  ],
  "total": 1
}
```

### DELETE `/backtest/:job_id`

取消回測任務，**只能取消 `pending` 狀態**（已開始執行的無法取消，`409 Conflict`）。

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

> **目前只有 `signal` 事件會真的被推播**（`signal.Engine.BroadcastFn`，在
> `cmd/server/main.go` 註冊）。`candle`/`indicator` 事件型別雖然定義在前端
> `ws/socket.ts`，但後端從未送出，一般情況（沒有觸發突破/爆量）下不會有
> 推播。前端 Dashboard 因此改用 REST（`/candles`、`/indicators`、`/signals`）
> 在頁面載入時主動 hydrate 監控清單欄位，WebSocket 只負責之後的訊號覆蓋更新。
