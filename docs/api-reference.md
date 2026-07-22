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
立刻確認某支股票當天有沒有觸發訊號，不用等 `daily_close` 排程（15:00 才對
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

### GET `/stock-symbols/search`

搜尋股票主檔，供 watchlist 新增股票時 autocomplete 使用。預設只回最近一次
TWSE ISIN 同步仍存在的標的。

**Query：**

| 參數 | 說明 |
|------|------|
| q | 代號或名稱關鍵字 |
| listed | 是否只查仍上市，預設 `true` |
| security_type | 依 TWSE ISIN 分類過濾，例如 `Stocks` / `ETFs` |
| limit | 回傳筆數，預設 20、上限 100 |

**Response：**
```json
{
  "symbols": [
    {
      "symbol": "2330",
      "name": "台積電",
      "isin_code": "TW0002330008",
      "market": "上市",
      "security_type": "Stocks",
      "industry": "半導體業",
      "is_listed": true
    }
  ]
}
```

### GET `/watchlist`

取得監控清單。回傳會附帶 `stock_symbol` 主檔狀態；`exists=false` 代表該
watchlist symbol 不在目前股票主檔內，`is_listed=false` 代表曾在主檔但最近
一次 TWSE ISIN 同步已未出現。

**Response：**
```json
{
  "watchlist": [
    {
      "symbol": "2330",
      "name": "台積電",
      "sector": "半導體業",
      "watched": true,
      "stock_symbol": {
        "exists": true,
        "is_listed": true,
        "isin_code": "TW0002330008",
        "market": "上市",
        "security_type": "Stocks",
        "industry": "半導體業"
      }
    }
  ]
}
```

### POST `/watchlist`

新增股票至監控清單。`name` / `sector` 可省略；省略時後端會從
`stock_symbols` 補股票名稱與產業。若 symbol 不在主檔且未提供 `name`，回 400。

**Request Body：**
```json
{ "symbol": "2330" }
```

### POST `/watchlist/bulk`

批次新增股票（已存在的 symbol 會更新名稱與產業）。可傳完整 `items`，也可只傳
`symbols` 讓後端從股票主檔補資料。

**Request Body：**
```json
{
  "symbols": ["2330", "2454"],
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

### POST `/scheduler/stock-symbol-sync/run`

手動觸發 TWSE ISIN 股票主檔同步，與每日 `stock_symbol_sync` 排程共用同一份邏輯。
此端點會立即回應，實際同步在背景執行；進度與結果可透過 `GET /scheduler/status`
查看 `stock_symbol_sync` 這個 job。

**Response（202 Accepted）：**
```json
{ "message": "stock_symbol_sync 已在背景重新觸發" }
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
  "end_date": "2024-12-31",
  "use_chip_filter": false,
  "chip_min_score": 0
}
```

`use_chip_filter` / `chip_min_score` 為選填。啟用後只對模組化策略生效，Python 端會用
`chip_scores.total_score` 逐 bar 過濾進場訊號；缺少該日籌碼資料時視為中性 `0`。
legacy `breakout_v1` 收到這兩個欄位時會忽略並記 warning log，不中斷回測。

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
    "use_chip_filter": false,
    "chip_min_score": 0,
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
{ "symbol": "2330", "timeframe": "1d", "limit": 250, "reuse_existing": false }
```

`timeframe` 省略時預設 `1d`；`limit` 為抓取的歷史K棒根數，省略或 0 時使用
Python 端預設值（250）。`reuse_existing` 預設 `false`，維持舊契約：每次呼叫
都重新分析並寫入一筆 DB 快照；只有明確傳 `true` 時，後端才會優先重用同
timeframe 且仍在重用期限內（目前 24 小時）的既有快照，找不到可重用快照才會
建立新分析。

**Response（201 Created）：**
```json
{
  "pipeline_version": "v2",
  "analysis": {
    "id": 4,
    "symbol": "2330",
    "timeframe": "1d",
    "analyzed_at": "2026-07-01T00:00:00+08:00",
    "current_price": 985.0,
    "model_version": "v4",
    "model_config_hash": "a1b2c3d4e5f6",
    "period_summaries": [{ "key": "short", "label": "短期", "support": {}, "resistance": null }],
    "analysis_tips": ["短期支撐守穩，接近區間時觀察量價確認。"],
    "chip_summary": { "missing": false, "score": 42.5, "signal": "BULLISH" },
    "created_at": "2026-07-01T10:00:00+08:00"
  },
  "features": {
    "global_trend": 0.032,
    "global_volatility": 0.018
  },
  "score": {
    "global_expected_value": 0.004,
    "global_confidence": 0.61,
    "global_risk_reward_ratio": 0.92
  },
  "evidence": {
    "trend": 0.032,
    "volatility": 0.018,
    "metrics": { "expected_value": 0.004, "confidence": 0.61, "risk_reward_ratio": 0.92 },
    "chip": { "missing": false, "score": 42.5, "signal": "BULLISH" },
    "model": {
      "version": "v4",
      "config_hash": "a1b2c3d4e5f6",
      "explainer": "permutation_shap",
      "explained_output": "calibrated_normalized_probability"
    }
  },
  "decision": {
    "action": "BuySmall",
    "action_label": "小量試單",
    "market_regime": {
      "primary": "TREND_UP",
      "flags": ["HIGH_VOLATILITY"],
      "label": "偏多趨勢但波動偏高",
      "reasons": ["整體趨勢 3.2%"]
    },
    "primary_zone": {
      "label": "960.00 ~ 970.00",
      "role": "SUPPORT",
      "distance_label": "1.5%",
      "trading_score": 78.5
    },
    "market_context": [],
    "confidence_explanation": {
      "value": 0.72,
      "level": "HIGH",
      "label": "72%（高）",
      "formula_factors": [],
      "context_factors": []
    },
    "risk_notes": ["波動偏高，倉位需保守。"],
    "secondary_zones": []
  },
  "explanation": {
    "schema_version": "sr_explain_v1",
    "summary": "2330 目前建議以「小量試單」解讀 SR Zone 結果。",
    "action_reason": "Action 為「小量試單」，主因是主交易區 960.00 ~ 970.00 目前被判定為支撐，交易分數 78.5。",
    "market_drivers": ["整體趨勢 +3.2%", "整體波動 1.8%", "整體信心 61%", "籌碼總分 42.5"],
    "risk_notes": ["波動偏高，倉位需保守。"],
    "model_context": {
      "version": "v4",
      "config_hash": "a1b2c3d4e5f6",
      "uses_shap_evidence": true
    }
  },
  "zones": [
    {
      "data": {
        "id": 16,
        "analysis_id": 4,
        "price_low": 960.0,
        "price_high": 970.0,
        "method": "atr",
        "role": "SUPPORT"
      },
      "features": {
        "support": { "touch_count": 4, "rejection_count": 3, "breakout_count": 0 },
        "resistance": { "touch_count": 4, "rejection_count": 1, "breakout_count": 1 }
      },
      "score": {
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
        "expected_value": 0.0272,
        "risk_reward_ratio": 2.29,
        "touch_count": 4,
        "support_touch_count": 3,
        "resistance_touch_count": 1,
        "recent_validation": "VALIDATED_RECENTLY",
        "trading_score": 78.5,
        "trading_score_breakdown": {
          "expected_value": 26.7,
          "risk_reward": 13.4,
          "trend": 10.0,
          "volume": 10.2,
          "confidence": 7.2,
          "chip": 11.0
        },
        "trading_recommendation": "BUY",
        "overlap_group": 0,
        "confluence_count": 2
      },
      "evidence": {
        "support": {
          "role": "SUPPORT",
          "targets": {
            "hold": {
              "baseline_probability": 0.50,
              "final_probability": 0.66,
              "additivity_error": 0.000002,
              "contributions": [
                {
                  "feature": "rejection_count",
                  "value": 3.0,
                  "contribution": 0.08,
                  "direction": "supportive"
                }
              ]
            }
          }
        },
        "resistance": {},
        "risk_flags": []
      },
      "explanation": {
        "schema_version": "sr_explain_v1",
        "role_summary": "960.00 ~ 970.00 位於現價下方或回測區，暫以支撐解讀。",
        "score_reason": "Trading Score 78.5 主要由期望值貢獻 26.7 分推動；最低分量是信心 7.2 分。",
        "probability_reason": "此區間目前按支撐解讀，反彈/守住機率為 66.0%，跌破/突破機率為 21.0%；期望值為 +2.72%。",
        "confidence_reason": "信心為 72%（高），主要參考目前角色方向樣本 3 次、整體觸碰 4 次、守住 3 次、跌破/突破 0 次；近期性為「最近有守住驗證」。",
        "positive_factors": ["信心等級高", "最近有有效驗證", "多方法共振 ×2"],
        "negative_factors": ["目前沒有明顯扣分因素"],
        "watch_conditions": ["觀察價格回測 960.00 ~ 970.00 時是否止跌", "若收盤跌破 960.00，支撐判斷失效風險升高"]
      },
      "lifecycle": {
        "status": "PENDING",
        "broken_at": null,
        "broken_price": null,
        "resolved_role": null
      }
    }
  ]
}
```

`role=AT_ZONE`（現價落在區間內）的 zone，`bounce_probability` 到
`volume_confirmation` 這些「已解析方向」才有意義的欄位一律是 `null`。
`trading_score_breakdown` 的六個分量加總即為 `trading_score`：EV(34%) + RR(17%) +
Trend(12.75%) + Volume(12.75%) + Confidence(8.5%) + Chip(15%)（見
sr-zone-scoring.md「十二」）。`zones` 陣列依 Tier 由粗到細排序，同層內依
`trading_score` 由高到低排序（`confluence_count` 只當第三順位 tie-
breaker，不改變主要排序規則）。`confidence` 依角色只用該方向
（`support_touch_count`/`resistance_touch_count` 其中之一）的樣本計算，見
sr-zone-scoring.md「六」。`overlap_group`/`confluence_count` 是跨方法重疊
分群結果，`overlap_group` 只有 `confluence_count > 1` 時才有值，見
sr-zone-scoring.md「十七」。

頂層依序對應 Data/Features/Score/Evidence/Decision。`analysis` 同時保存
`period_summaries`、`analysis_tips` 與專屬 `chip_summary`；`decision` 是決策
摘要，`explanation` 是 deterministic 白話解釋層。每個 zone 也分成
`data/features/score/evidence/explanation/scenario/lifecycle`，驗證 API 只更新
lifecycle。`score` 只帶評分欄位；zone 的識別（id/price_low/method/role）在
`data`、生命週期（status/broken_at…）在 `lifecycle`、
`features/evidence/explanation/scenario` 各自為兄弟鍵，不在 `score` 內重複。
欄位語意見 sr-zone-scoring.md「十四、十九」。

`explanation` 不取代 `evidence`：前者給前端直接呈現白話結論、加分/扣分因素與
風險提醒；後者保留 SHAP baseline、最終機率與特徵貢獻等進階模型證據。舊分析
可能沒有 explanation，客戶端應回退顯示 `decision`、`analysis.analysis_tips`
與既有 evidence。

**Explanation 欄位：**

頂層 `explanation`：

| 欄位 | 型別 | 說明 |
|---|---|---|
| `schema_version` | string | Explanation schema 版本，目前為 `sr_explain_v1` |
| `summary` | string | 一句整體白話結論，對齊 `decision.action` |
| `action_reason` | string | 為什麼得到目前 action |
| `market_drivers` | string[] | 趨勢、波動、信心、籌碼等主要因素 |
| `risk_notes` | string[] | 風險提醒；通常整合 decision risk notes 與全局風險 |
| `model_context.version` | string | 產生解釋時使用的模型版本 |
| `model_context.config_hash` | string | 模型訓練設定 hash |
| `model_context.uses_shap_evidence` | boolean | 本次 explanation 是否可引用 SHAP evidence |

每個 `zones[].explanation`：

| 欄位 | 型別 | 說明 |
|---|---|---|
| `schema_version` | string | Explanation schema 版本，目前為 `sr_explain_v1` |
| `role_summary` | string | 支撐、壓力或 `AT_ZONE` 方向未定的白話描述 |
| `score_reason` | string | trading score 的最高與最低分量說明 |
| `probability_reason` | string | 反彈/跌破機率、期望值的解釋；`AT_ZONE` 不給方向性結論 |
| `confidence_reason` | string | 樣本數、近期性、守住/跌破穩定度如何影響 confidence |
| `positive_factors` | string[] | 加分因素 |
| `negative_factors` | string[] | 扣分或風險因素 |
| `watch_conditions` | string[] | 後續要觀察的價位、量能、突破或跌破條件 |

`explanation` 是 deterministic template output，不是 LLM 文字。客戶端可以直接顯示，
但不應把它當成新的 scoring 欄位或交易門檻。

**籌碼摘要欄位**（見 sr-zone-scoring.md「十二之一」）：`analysis.chip_summary`
是整檔層級的籌碼拆解，`score`/`institutional_score`/`margin_score`/`broker_score`
為 −100~100、`concentration_score` 為 0~100、`signal` 為
`BULLISH`/`BEARISH`/`NEUTRAL`/`RISK`。查無籌碼資料時 `chip_summary.missing=true`
且各分數為 `null`（跟「分數接近 0 的中性」不同）；更舊、尚未帶此欄位的分析
則整個 `chip_summary` 為 `null`。新分析的 `evidence.chip` 使用同一份計算結果；
舊分析沒有 evidence 時，客戶端應回退讀取 `analysis.chip_summary`。

每張摘要卡另在 `period_summaries[].support`／`.resistance` 底下帶一個角色化的
`chip` 物件：

```json
"chip": {
  "direction": "bullish",        // bullish / bearish / neutral / none（none=無資料）
  "contribution": 11.0,          // 籌碼對這個角色 trading_score 的直接加權貢獻（0~15，已依支撐/壓力翻號）
  "bounce_delta_pp": 6.2,        // 籌碼對本 zone 反彈（hold）機率的模型邊際貢獻（百分點）；無資料時 null
  "break_delta_pp": -3.0         // 籌碼對本 zone 跌破/突破機率的模型邊際貢獻（百分點）；無資料時 null
}
```

`contribution`（直接加權）與 `*_delta_pp`（v4 模型特徵）是籌碼影響分數的兩條
獨立路徑，不是重複計分。前端摘要卡對支撐顯示 `bounce_delta_pp`（反彈守住）、
對壓力顯示 `break_delta_pp`（突破壓力），兩者是不同事件。

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

**Response：** 格式同 `GET /sr-zones/:id`，但 `zones[].lifecycle` 會反映最新
驗證結果：
```json
{
  "pipeline_version": "v2",
  "analysis": { "id": 4, "symbol": "2330" },
  "features": {},
  "score": {},
  "evidence": {},
  "decision": {},
  "zones": [
    {
      "data": { "id": 16, "price_low": 960.0, "price_high": 970.0 },
      "features": {},
      "score": {},
      "evidence": {},
      "lifecycle": {
        "status": "BROKEN",
        "broken_at": "2026-07-05T00:00:00+08:00",
        "broken_price": 940.0,
        "resolved_role": null
      }
    }
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
{
  "symbols": ["2330", "2454"],
  "timeframe": "1d",
  "limit": 1500,
  "model_type": "gradient_boosting",
  "split_method": "time",
  "calibration_method": "sigmoid"
}
```

`symbols` 省略或空陣列時自動使用整個監控清單（watchlist 為空則回
`400`）；`limit` 為每檔股票訓練用的歷史K棒根數（預設 1500）；`model_type`
可選 `gradient_boosting`（預設）、`hist_gradient_boosting`、`lightgbm` 或
`logistic_regression`。`split_method` 可選 `time`（預設，正式評估建議）或
`random`（舊行為，僅建議比較）；`calibration_method` 可選 `sigmoid`（預設）、
`isotonic` 或 `none`。

目前系統只維持一個現行模型；訓練成功會覆蓋 `SR_SCORING_MODEL_PATH` 指向的
active model。`sr_scoring_train_jobs` 是訓練任務紀錄，不是可切換的模型清單。

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
      "model_path": "models/sr_scoring_v4.joblib",
      "model_version": "v4",
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

### DELETE `/sr-zones/train-jobs`

清理舊的訓練任務紀錄，只刪除 `done` / `failed`，不刪 `pending` / `running`。

**Query Parameters：** `keep`（預設 20；小於 5 會提升為 5，最多 200）

**Response：**
```json
{ "deleted": 12, "keep": 20 }
```

### GET `/sr-zones/model-status`

查詢目前機率模型的狀態，讓前端在觸發分析前就能知道模型準備好了沒——
**永遠回 200**，不像 `POST /sr-zones` 那樣在模型不存在時回 503，用
`exists` 欄位表示狀態。

**Response（模型存在）：**
```json
{
  "exists": true,
  "version": "v4",
  "trained_at": "2026-07-01T13:30:00+08:00",
  "model_path": "models/sr_scoring_v4.joblib",
  "split_method": "time",
  "metrics": { "hold": { "auc": 0.81, "calibrated": 1.0 }, "break": { "auc": 0.77, "calibrated": 1.0 } },
  "feature_names": ["touch_count", "rejection_count", "..."],
  "config_hash": "a1b2c3d4e5f6",
  "training_config": {
    "dataset_config": { "forward_bars_support": 5, "threshold_pct_support": 0.03 },
    "zone_builders": { "ATRZoneBuilder": { "atr_width_multiplier": 1.5 }, "VolumeProfileZoneBuilder": { "num_bins": 24 } },
    "model_type": "gradient_boosting", "split_method": "time", "calibration_method": "sigmoid"
  }
}
```
`config_hash`/`training_config` 見 sr-zone-scoring.md「十六」——`config_hash`
跟分析快照的 `model_config_hash` 是同一個值，可以用來確認「現在的模型」
跟「某筆舊分析用的模型」是不是同一組訓練設定。

**Response（模型不存在）：**
```json
{
  "exists": false, "version": null, "trained_at": null, "model_path": null,
  "split_method": null, "metrics": null, "feature_names": null,
  "config_hash": null, "training_config": null
}
```

### DELETE `/sr-zones/:id`

刪除一筆分析紀錄（連同其 zones 一併刪除）。

---

## Chip API

籌碼分析 API 使用已同步到 DB 的三大法人、融資融券、券商分點與 `chip_scores`。
資料同步由 `POST /chips/sync` 建立非同步 job；收盤後 `daily_close` 也會另外跑
`chip_daily_sync`，其紀錄在 `job_runs`，不是 `chip_sync_jobs`。

### GET `/chips/:symbol/summary`

查詢單一股票籌碼摘要。`date` 省略時回傳最新一筆 `chip_scores`；若指定日期但查無
分數，回 `404`。

**Query Parameters：** `date`（選填，`YYYY-MM-DD`）

**Response：**
```json
{
  "symbol": "2330",
  "date": "2026-07-03",
  "signal": "BULLISH",
  "totalScore": 72.5,
  "reason": ["外資連續買超 4 日"],
  "institutional": {
    "foreignNetBuy": 12000,
    "investmentTrustNetBuy": 3000,
    "dealerNetBuy": -500,
    "consecutiveDays": 4
  },
  "margin": {
    "marginBalance": 23000,
    "marginChange": -1200,
    "shortBalance": 4200,
    "shortChange": 800
  },
  "broker": {
    "topNetBuy": 9000,
    "concentration": 0.18
  }
}
```

`institutional` / `margin` / `broker` 會各自獨立查詢；某區塊查無資料時省略該區塊，
不會讓整個 summary 失敗。

### GET `/chips/:symbol/scores`

查詢歷史籌碼分數。

**Query Parameters：** `from`、`to`（必填，`YYYY-MM-DD`）

**Response：**
```json
{ "symbol": "2330", "scores": [ { "trade_date": "2026-07-03T00:00:00+08:00", "total_score": 72.5, "...": "..." } ] }
```

### GET `/chips/:symbol/brokers`

查詢券商分點買賣超排行。

**Query Parameters：**

| 參數 | 預設 | 說明 |
|------|------|------|
| date | 必填 | `YYYY-MM-DD` |
| limit | `20` | 1～200，超出範圍會退回 20 |

**Response：**
```json
{ "symbol": "2330", "date": "2026-07-03", "topBuy": [ { "...": "..." } ], "topSell": [ { "...": "..." } ] }
```

### POST `/chips/sync`

手動同步籌碼資料，立即建立 `chip_sync_jobs` 紀錄並背景執行。

**Request Body：**
```json
{
  "mode": "manual",
  "symbols": ["2330", "2317"],
  "from": "2026-07-01",
  "to": "2026-07-03",
  "dataTypes": ["institutional", "margin", "broker", "scores"],
  "force": false
}
```

`mode` 可為 `manual` 或 `backfill`，省略時為 `manual`。`manual` 未指定日期時只同步
今天；`backfill` 未指定 `from` 時會使用 `chip.sync.history_trading_days` 往回推。
`force` 目前會記錄在 job，但 upsert 本身已具冪等性，尚未實作跳過既有資料的特殊邏輯。

**Response（202 Accepted）：**
```json
{
  "job": {
    "job_id": "chip_20260708_120000_000",
    "mode": "manual",
    "status": "pending",
    "symbols_total": 2,
    "symbols_done": 0,
    "symbols_failed": 0
  }
}
```

### GET `/chips/sync/:job_id`

查詢 manual/backfill 籌碼同步任務。

**Response：**
```json
{
  "job": {
    "job_id": "chip_20260708_120000_000",
    "status": "done",
    "symbols_done": 2,
    "symbols_failed": 0,
    "failures": []
  }
}
```

`status` 可為 `pending`、`running`、`done`、`partial`、`failed`。找不到 job 回 `404`。

---

## Trade Analysis API

Trade Analysis 是新前端與 API 的統一交易決策入口。呼叫端只需要提供股票代號；
後端會自動讀取 `positions` projection，若資料庫沒有持股資料或股數為 0，就以
`FLAT` 空手情境分析；若有股數則以 `LONG` 持股情境分析。

- `POST /trade-analysis/analyze`：body 為
  `{"symbol":"2330","timeframe":"1d","limit":250,"force_refresh":false}`。
- `GET /trade-analysis/:symbol/history?limit=20`：列出該股票 FLAT/LONG 共用分析歷史。

`POST /trade-analysis/analyze` 回應：

```json
{
  "context": {
    "symbol": "2330",
    "position_state": "FLAT",
    "has_position": false
  },
  "analysis": {},
  "sr_zone_analysis": {},
  "zones": []
}
```

`analysis` 沿用 Position Analysis 快照格式；`sr_zone_analysis` 與 `zones` 沿用
SR Zone 快照格式。分析入口統一由 `/trade-analysis/*` 提供（舊的
`/position-analyses` 分析 endpoints 已移除）。

`analysis.sr_zone_analysis_id` 是 best-effort historical reference。若對應 SR Zone
快照後來被刪除，trade-analysis 歷史仍會保留 `analysis` 內的決策快照欄位，但
`sr_zone_analysis` / `zones` 可能無法再由該 id 回查完整市場結構明細。

---

## Position Analysis API

Position Analysis 是 Trade Analysis 背後的決策快照與部位帳務 API。沒有
transaction/projection 時視為 `FLAT`，有股數時為 `LONG`。交易決策入口統一為
`/trade-analysis/*`；以下 endpoints 提供 position ledger 與 projection。

- `GET /positions`：列出目前 LONG positions。
- `GET /positions/:symbol`：取得 projection；空手回傳股數、AVG、version 均為 0。
- `GET /positions/:symbol/transactions`：取得 immutable ledger。
- `POST /positions/:symbol/transactions`：新增 BUY/SELL；body 包含
  `event_type`、`shares`、`price`、`fee`、`tax`、`occurred_at`、
  `expected_version`、`note`。SELL 不得超賣。
- `POST /positions/:symbol/adjustments`：新增 ADJUSTMENT；body 包含更正後
  `target_shares`、`target_avg_cost`、`expected_version` 與必填 `reason`。
  ADJUSTMENT 只校正 projection，不代表成交、不改變 `realized_pnl`；實際交易使用
  BUY/SELL transaction。

分析歷史改由 `GET /trade-analysis/:symbol/history` 取得；分析輸出包含 `position_state`、Position version、目前／目標／調整股數、
`adjustment_side`/`adjustment_amount`、Action、進場／停損／停利價、風險金額、
預期報酬、RR、已實現／未實現損益、設定快照、Evidence、觸發與失效條件。

固定預設為：單股上限 200,000、最大風險 10,000、加碼 tranche 25%、
最低 RR 1.5、突破後無上方壓力時以 2R 推導停利目標、停利減碼 50%。設定由
`backend/config.yaml::position_analysis` 覆寫。分析 evidence 的
`take_profit_source` 為 `RESISTANCE_ZONE` 或 `BREAKOUT_R_MULTIPLE`，可區分停利價來源。

## Legacy Holdings API（已移除）

`/holdings*` 與 `/holding-analyses*` routes 已由 migration 038 移除，不應再由
新客戶端呼叫。舊 `holdings` 會依 symbol 轉成一筆 `OPENING_BALANCE`
transaction 與 `positions` projection；舊 `holding_analyses` 會搬到
`position_analyses`，並以 `rule_version=holding_sr_zone_v1_legacy` 保留歷史來源。

新流程請使用 `/positions*` 管理 immutable ledger / AVG projection，並使用
`/trade-analysis/*` 或相容的 `/position-analyses*` 產生分析快照。

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
