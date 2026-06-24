# Python ↔ Go Integration Specification

## System Overview

```
[Python]  ← Research / Backtest Layer
   ↑  ↓  （共用同一個 DB）
[Go]      ← Production / Real-time Layer
```

---

# 1. Role Separation

## Go（Production Layer）

- 即時行情處理（FinMind 輪詢 / 未來 Shioaji）
- 技術指標計算（MA / RSI / MACD / VWAP / ATR / Bollinger）
- Breakout / Breakdown 判斷
- Signal 生成與 WebSocket 推播
- Watchlist 掃描（~1900 檔）
- 回測任務管理（寫入 backtest_jobs）

## Python（Research Layer）

- 策略研究與回測（backtrader）
- 指標驗證（與 Go 1:1 對齊）
- 統計分析
- 回測結果寫回 DB（backtest_results + backtest_trades）

---

# 2. Go → Python 驅動方式

## 方式 A：DB Polling（已實作，預設）

```
Go 寫入 backtest_jobs（status='pending'）
        ↓
Python worker 每 10 秒掃描 pending 任務
        ↓
執行 backtrader 回測
        ↓
寫入 backtest_results + backtest_trades
        ↓
更新 backtest_jobs.status = 'done'
```

啟動：`python worker.py`

## 方式 B：HTTP Service（已實作，可選）

```
Go POST /backtest → Python FastAPI（port 8001）
Python 執行回測（同步）
Python 回傳結果 + 寫回 DB
```

啟動：`uvicorn http_server:app --port 8001`  
Go 端需在 `config.yaml` 設定 `python.service_url: http://localhost:8001`。

---

# 3. Job Schema

寫入 `backtest_jobs` 資料表：

```json
{
  "job_id": "bt_20260624_001",
  "type": "backtest",
  "strategy": "breakout_v1",
  "symbols": ["8088", "2399"],
  "timeframe": "1d",
  "start_date": "2023-01-01",
  "end_date": "2026-06-01",
  "status": "pending",
  "trigger": "manual"
}
```

---

# 4. Backtest Data Standard

## 核心原則

> Python 與 Go 必須使用同一份 DB schema，讀取同一份 candles 資料。

## Candle Schema（正式定義）

```go
type Candle struct {
    Symbol    string
    Timeframe string
    Open      float64
    High      float64
    Low       float64
    Close     float64
    Volume    int64
    Amount    float64
    Timestamp int64  // Unix timestamp
}
```

## Python DataFrame 欄位對應

```python
columns = ["symbol", "timeframe", "open", "high", "low",
           "close", "volume", "amount", "timestamp"]
```

timestamp 取法：
- SQLite：`CAST(strftime('%s', ts) AS INTEGER)`
- MySQL：`UNIX_TIMESTAMP(ts)`
- PostgreSQL：`EXTRACT(EPOCH FROM ts)::BIGINT`

---

# 5. Strategy Consistency Rule

> Python 回測邏輯必須與 Go production 邏輯 1:1 對齊。

禁止：
- Python 使用不同的 MA 計算方式
- Python 加入 Go 沒有的平滑處理
- 不同的成交量平均窗口

必須共用的參數：
- MA window（5 / 10 / 20 / 60）
- Volume average window（20）
- Breakout 條件（Close > Resistance, VolRatio >= 2.0, Trend == BULLISH）
- Support/Resistance 識別邏輯（window=3, merge threshold=1%）

---

# 6. 資料庫設定同步

`backend/config.yaml` 與 `python/config.yaml` 的 `database` 區段需指向同一個 DB。  
Docker Compose 環境下透過環境變數統一設定，不需手動同步。

---

# 7. Design Philosophy

- Go = real-time decision engine
- Python = truth validation engine
- DB = historical truth source
- Strategy correctness > performance
