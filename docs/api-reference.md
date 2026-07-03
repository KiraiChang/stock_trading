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

### POST `/indicators/:symbol/compute`

手動計算單一股票的最新指標快照並寫入 DB，**不要求該股票在監控清單裡**，
只要求 `candles` 至少有 35 根（`timeframe` 對應的週期）。同步執行、直接
回傳算出來的結果，用來補算「有 candles 但從未被排程算過指標」的股票
（例如剛用 backfill 拉完歷史資料、但還沒加進監控清單的股票）。

**Query Parameters：** `timeframe`（預設 `1d`）

**Response（200）：** 格式同 `GET /indicators/:symbol`。

**錯誤：** candles 不足 35 根時回傳 `422 Unprocessable Entity`。

前端「歷史資料回補」頁面（`/backfill`）下方有「手動計算指標」區塊，輸入任意
股票代號即可觸發，不需要透過 API 手動呼叫。

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

### POST `/signals/:symbol/evaluate`

手動觸發訊號評估。完全基於 `candles`（OHLCV）計算——內部會先呼叫指標計算
（同 `/indicators/:symbol/compute`），再做支撐/壓力/趨勢判斷與
`CheckBreakout`——不需要即時行情、不要求該股票在監控清單裡，適合**收盤後**
立刻確認某支股票當天有沒有觸發訊號，不用等 `daily_close` 排程（14:00 才對
監控清單跑）。

**Query Parameters：** `timeframe`（預設 `1d`）

**Response（200，有觸發）：**
```json
{ "signal": { "id": 1, "symbol": "2330", "signal_type": "BREAKOUT", "direction": "BUY", "...": "..." } }
```

**Response（200，沒有觸發）：**
```json
{ "signal": null, "message": "沒有觸發訊號（不符合突破/跌破/爆量條件）" }
```

**錯誤：** candles 不足 35 根時回傳 `422 Unprocessable Entity`。

前端「歷史資料回補」頁面（`/backfill`）下方有「手動評估訊號」區塊，輸入任意
股票代號即可觸發，不需要透過 API 手動呼叫。

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

### PATCH `/watchlist/:symbol/watch`

設定或取消該股票的**即時監聽**（是否透過 WebSocket 推播）。監控清單本身可以
很大，但即時監聽刻意限制**同時最多 3 檔**（`store.MaxWatchedSymbols`），跟這
套系統「非高頻」的定位一致；前端只會對監聽中的股票送出 WebSocket 訂閱。

**Request Body：**
```json
{ "watched": true }
```

**Response（200）：**
```json
{ "symbol": "2330", "watched": true }
```

**錯誤：** 已有 3 檔在監聽時，再設定第 4 檔會回傳 `409 Conflict`：
```json
{ "error": "已達監聽上限（3 檔），請先取消其他股票的監聽" }
```

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

## Stock Analysis API

針對單一個股，用歷史 OHLCV 算出支撐/壓力/進場/停損/停利，供人工判斷用
（不是自動下單訊號）。實際計算由 Python 完成（重用
[backtest-modular-strategy.md](./backtest-modular-strategy.md) 的模組化元件），
**需要 `python.service_url` 已設定且 Python HTTP service 已啟動**，否則
`POST /analysis` 會回傳 `502 Bad Gateway`。驗證（`POST /analysis/:id/verify`）
不依賴 Python，純粹比對 Go 這邊的 `candles` 表，Python 沒開也能用。
完整規格見 [stock-analysis.md](./stock-analysis.md)。

### POST `/analysis`

觸發一次分析並寫入 DB。

**Request Body：**
```json
{ "symbol": "2330", "timeframe": "1d" }
```

`timeframe` 省略時預設 `1d`。

**Response（201 Created）：**
```json
{
  "analysis": {
    "id": 1,
    "symbol": "2330",
    "timeframe": "1d",
    "analyzed_at": "2026-07-01T00:00:00+08:00",
    "current_price": 978.0,
    "trend": "BULLISH",
    "entry_status": "WATCHING",
    "entry_direction": "LONG",
    "entry_price": 985.0,
    "entry_reason": "等待突破壓力 985.00（來源：swing）",
    "stop_loss_atr": 960.2,
    "stop_loss_structural": 965.0,
    "stop_loss_composite": 965.0,
    "take_profit_next_level": 1020.0,
    "take_profit_risk_reward": 1025.0,
    "take_profit_atr": 1030.5,
    "trade_verification": null,
    "verified_at": null,
    "created_at": "2026-07-01T10:00:00+08:00"
  },
  "levels": [
    { "id": 1, "analysis_id": 1, "price": 985.0, "type": "RESISTANCE", "strength": 1.0, "method": "swing", "status": "PENDING" },
    { "id": 2, "analysis_id": 1, "price": 955.0, "type": "SUPPORT", "strength": 0.9, "method": "volume_profile_poc", "status": "PENDING" }
  ]
}
```

### GET `/analysis`

列出歷史分析紀錄。

**Query Parameters：** `symbol`（篩選特定股票）、`limit`（預設 20，最多 200）

### GET `/analysis/:id`

取得單筆分析詳情（含支撐/壓力清單），格式同 `POST /analysis` 的回應。

### POST `/analysis/:id/verify`

手動重新驗證：比對這筆分析之後的實際 K 棒，更新每個支撐/壓力位的
`status`（是否被突破），以及（若 `entry_status=ACTIVE`）三種停損/三種停利
各自有沒有被觸及。**可重複呼叫**，每次都用目前為止最新的資料重新計算，
不是一次性判定。沒有自動排程，需要主動呼叫這支 API 才會更新。

**Response：** 格式同 `GET /analysis/:id`，但 `trade_verification` 會有值：
```json
{
  "analysis": { "...": "...", "trade_verification": "{\"applicable\":true,\"stop_loss\":{...},\"take_profit\":{...}}", "verified_at": "2026-07-05T09:00:00+08:00" },
  "levels": [ { "...": "...", "status": "BROKEN", "broken_at": "2026-07-03T00:00:00+08:00", "broken_price": 950.0 } ]
}
```

### DELETE `/analysis/:id`

刪除一筆分析紀錄（連同其支撐/壓力位一併刪除）。前端「個股分析」頁面的
歷史紀錄列表提供刪除按鈕（會先跳出確認列，比照監控清單的刪除確認方式）。

---

## SR Zone Scoring API

機構級支撐/壓力機率評分——輸出**價格區間（zone）**而非單一價位，每個 zone
帶有機率模型算出的反彈/跌破機率、期望值、風險報酬比、可拆解的交易分數等。
跟 Stock Analysis API 是完全獨立的兩套系統，不要混淆。完整演算法規格見
[sr-zone-scoring.md](./sr-zone-scoring.md)。

**需要 `python.service_url` 已設定、Python HTTP service 已啟動、且機率模型
已訓練過**（`POST /sr-zones/train` 或 CLI `python -m
backtest.modular.sr_scoring.train`），否則 `POST /sr-zones` 會回傳
`502 Bad Gateway`（Python service 沒開）或模型未訓練時的錯誤（fail-fast，
不會靜默回傳中性機率）。`status`/`broken_at`/`broken_price` 由
`POST /sr-zones/:id/verify` 更新（見下方），或由 `daily_close` 排程每天
自動對最近幾筆分析重新驗證一次。

### POST `/sr-zones`

觸發一次分析並寫入 DB。

**Request Body：**
```json
{ "symbol": "2330", "timeframe": "1d", "limit": 250 }
```

`timeframe` 省略時預設 `1d`；`limit` 為抓取的歷史K棒根數，省略或 0 時使用
Python 端預設值（250）。

**Response（201 Created）：**
```json
{
  "analysis": {
    "id": 4,
    "symbol": "2330",
    "timeframe": "1d",
    "analyzed_at": "2026-07-01T00:00:00+08:00",
    "current_price": 985.0,
    "global_trend": 0.032,
    "global_volatility": 0.018,
    "global_expected_value": 0.004,
    "global_confidence": 0.61,
    "global_risk_reward_ratio": 0.92,
    "model_version": "v2",
    "created_at": "2026-07-01T10:00:00+08:00"
  },
  "zones": [
    {
      "id": 16,
      "analysis_id": 4,
      "price_low": 960.0,
      "price_high": 970.0,
      "method": "atr",
      "role": "SUPPORT",
      "tier": "TIER_1_MAIN_STRUCTURE",
      "tier_label": "主結構",
      "support_score": 0.68,
      "resistance_score": 0.30,
      "net_score": 0.38,
      "net_score_label": "STRONG_SUPPORT",
      "confidence": 0.72,
      "confidence_level": "HIGH",
      "bounce_probability": 0.66,
      "break_probability": 0.21,
      "expected_gain": 0.048,
      "expected_loss": -0.021,
      "expected_value": 0.0272,
      "risk_reward_ratio": 2.29,
      "reward_risk_percentile": 78.0,
      "relative_volume": 1.4,
      "volume_confirmation": "CONFIRMED",
      "touch_count": 4,
      "reject_count": 3,
      "break_count": 0,
      "zone_momentum": 0.021,
      "zone_direction": "UP",
      "recent_validation": "VALIDATED_RECENTLY",
      "trading_score": 78.5,
      "trading_score_breakdown": { "expected_value": 30.0, "risk_reward": 15.3, "trend": 10.6, "volume": 15.0, "confidence": 7.2 },
      "trading_recommendation": "BUY",
      "status": "PENDING",
      "broken_at": null,
      "broken_price": null
    }
  ]
}
```

`role=AT_ZONE`（現價落在區間內）的 zone，`bounce_probability` 到
`volume_confirmation` 這些「已解析方向」才有意義的欄位一律是 `null`。
`trading_score_breakdown` 的五個分量加總即為 `trading_score`（見
sr-zone-scoring.md「十二」）。`zones` 陣列依 Tier 由粗到細排序，同層內依
`trading_score` 由高到低排序。

### GET `/sr-zones`

列出歷史分析紀錄。

**Query Parameters：** `symbol`（篩選特定股票）、`limit`（預設 20，最多 200）

### GET `/sr-zones/:id`

取得單筆分析詳情（含 zones 清單），格式同 `POST /sr-zones` 的回應。

### POST `/sr-zones/:id/verify`

手動重新驗證：比對這筆分析之後的實際 K 棒，更新每個 zone 的 `status`
（是否被突破）。**可重複呼叫**，每次都用目前為止最新的資料重新計算，
不是一次性判定；`daily_close` 排程也會每天自動對最近幾筆分析呼叫一次
（見 sr-zone-scoring.md「十四」）。

**Response：** 格式同 `GET /sr-zones/:id`，但 zones 的 `status`/`broken_at`/
`broken_price` 會反映最新驗證結果：
```json
{
  "analysis": { "...": "..." },
  "zones": [
    { "...": "...", "status": "BROKEN", "broken_at": "2026-07-05T00:00:00+08:00", "broken_price": 940.0 },
    { "...": "...", "status": "HELD_SO_FAR", "broken_at": null, "broken_price": null }
  ]
}
```

`role=AT_ZONE` 的 zone 在分析當下現價落在區間內、方向未定，會維持
`PENDING` 直到後續某根K棒收盤真正離開區間才開始判斷突破；`BROKEN` 的 zone
不會因為後續反彈被改回 `HELD_SO_FAR`（沒有另外設計「重置」API）。

### POST `/sr-zones/train`

觸發 `hold_model`/`break_model` 重新訓練。**非同步**——立即建立一筆
`sr_scoring_train_jobs` 紀錄並回傳 `job_id`（`status=pending`），實際訓練在
背景 goroutine 執行（視資料量可能耗時數十秒到數分鐘），依序更新
`pending → running → done`/`failed`。用 `job_id` 輪詢
`GET /sr-zones/train-jobs/:job_id` 查詢進度，不需要只靠伺服器 log 或重新
呼叫 `POST /sr-zones` 猜測新模型是否已生效。

**Request Body：**
```json
{ "symbols": ["2330", "2454"], "timeframe": "1d", "limit": 1500, "model_type": "gradient_boosting" }
```

`symbols` 省略或空陣列時自動使用整個監控清單（watchlist 為空則回
`400`）；`limit` 為每檔股票訓練用的歷史K棒根數（預設 1500）；`model_type`
可選 `gradient_boosting`（預設）或 `logistic_regression`。

**Response（202 Accepted）：**
```json
{ "job_id": "sr_train_20260703_090000_000", "status": "pending", "message": "模型訓練已在背景啟動", "symbols": 12 }
```

### GET `/sr-zones/train-jobs`

列出最近的訓練任務。

**Query Parameters：** `limit`（預設 20，最多 200）

**Response：**
```json
{
  "jobs": [
    {
      "id": 3,
      "job_id": "sr_train_20260703_090000_000",
      "status": "done",
      "symbols": "[\"2330\",\"2454\"]",
      "timeframe": "1d",
      "fetch_limit": 1500,
      "model_type": "gradient_boosting",
      "rows": 128,
      "sources": 2,
      "split_method": "time",
      "metrics": {
        "hold": { "auc": 0.81, "accuracy": 0.76, "brier_score": 0.18, "log_loss": 0.52, "calibrated": 1.0, "train_rows": 102, "test_rows": 26, "positive_rate_train": 0.48, "positive_rate_test": 0.5 },
        "break": { "auc": 0.77, "accuracy": 0.72, "brier_score": 0.21, "log_loss": 0.58, "calibrated": 1.0, "train_rows": 102, "test_rows": 26, "positive_rate_train": 0.31, "positive_rate_test": 0.35 }
      },
      "model_path": "models/sr_scoring_v2.joblib",
      "model_version": "v2",
      "dataset_summary": {
        "rows": 128, "rows_by_symbol": { "2330": 90, "2454": 38 },
        "role_counts": { "SUPPORT": 70, "RESISTANCE": 58 },
        "hold_positive_rate": 0.49, "break_positive_rate": 0.33,
        "feature_zero_rate": { "breakout_count": 0.62, "touch_count": 0.0 },
        "rr_reference_count": 41
      },
      "error": null,
      "started_at": "2026-07-03T09:00:01+08:00",
      "finished_at": "2026-07-03T09:01:47+08:00",
      "created_at": "2026-07-03T09:00:00+08:00"
    }
  ],
  "total": 1
}
```

`rows`/`sources`/`metrics`/`model_path`/`model_version`/`dataset_summary`
只有 `status=done` 才有值；`error` 只有 `status=failed` 才有值。
`split_method` 是 `"time"`（預設，依 `touch_time` 逐股票切分 holdout）或
`"random"`（舊行為）；`metrics.calibrated` 是 `1.0`/`0.0`，訓練集太小時會
自動降級為不校準（見 sr-zone-scoring.md「四」）。

### GET `/sr-zones/train-jobs/:job_id`

取得單筆訓練任務詳情，格式同上方陣列裡的單一物件（`{ "job": {...} }`）。
找不到回 `404`。

### DELETE `/sr-zones/:id`

刪除一筆分析紀錄（連同其 zones 一併刪除）。

---

## WebSocket

**連線：** `ws://localhost:8080/ws/market`

**訂閱：**
```json
{ "action": "subscribe", "symbols": ["2330", "2454"] }
```

同時訂閱檔數**最多 3 檔**（跟 watchlist 的 `watched` 欄位上限一致，見
Watchlist API 的 `PATCH /watchlist/:symbol/watch`）；超過上限的 symbol 會被
忽略並記一筆 server log，不會回錯誤給 client（目前協定沒有 ack/error 訊息
機制）。真正的把關點是 `watched` 欄位——前端只會對監聽中的股票送出
subscribe，這裡的檔數上限只是防禦性的第二層保護。

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
