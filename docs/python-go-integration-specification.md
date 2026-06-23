# Python ↔ Go Integration Specification

## System Overview

本系統採用雙層架構：

```text id="arch_001"
[Python]  ← Research Layer
   ↑  ↓
   │  │
[Go]      ← Production Layer
```

---

# 1. Role Separation

## Go (Production Layer)

負責：

* 即時行情處理
* 技術指標計算（MA / RSI / MACD / VWAP）
* Breakout / Stop Loss 判斷
* Signal generation
* Notification
* Watchlist scanning（~1900 stocks）

特性：

* High concurrency (goroutines)
* Low latency
* Always-on service

---

## Python (Research Layer)

負責：

* Strategy research
* Backtesting
* Indicator validation
* Statistical analysis
* Strategy prototyping

特性：

* Batch processing
* Offline computation
* Experiment-driven

---

# 2. Go → Python 驅動方式

Go 不直接嵌入 Python 邏輯，而是透過「任務式驅動」。

---

## 方式 A：File-based Job (推薦)

### Flow

```text id="flow_001"
Go 產生回測任務
        ↓
寫入 JSON file / DB
        ↓
Python worker poll / consume
        ↓
執行回測
        ↓
回寫結果 (MySQL / JSON)
```

---

### Job Schema

```json id="job_001"
{
  "job_id": "bt_20260623_001",
  "type": "backtest",
  "strategy": "breakout_v1",
  "symbols": ["8088", "2399"],
  "timeframe": "1d",
  "start_date": "2023-01-01",
  "end_date": "2026-06-01"
}
```

---

## 方式 B：HTTP Service (可選)

Python running as service:

```text id="http_001"
Go → POST /backtest → Python API
Python → return result
```

適合：

* 即時策略測試
* 小規模分析

---

## 方式 C：Queue-based (進階)

```text id="queue_001"
Go → Redis Queue / Kafka
Python worker consume
```

適合：

* 大規模回測
* 批次策略掃描

---

# 3. Python → Go 回寫方式

Python 不直接影響 production system，只輸出：

## Strategy Definition Contract

```json id="strategy_001"
{
  "strategy_name": "breakout_v1",
  "rules": {
    "close_above_resistance": true,
    "volume_multiplier": 2.0,
    "ma_filter": "MA20"
  },
  "metrics": {
    "win_rate": 0.62,
    "max_drawdown": -0.08,
    "return": 0.18
  }
}
```

---

Go 讀取後：

* 轉換為 production rule
* 編譯進 signal engine

---

# 4. Backtest Data Standard (非常重要)

## Core Principle

> Python 與 Go 必須使用同一份 data schema

---

## 4.1 Candle Schema (Canonical)

```go id="candle_001"
type Candle struct {
    Symbol    string
    Timeframe string

    Open      float64
    High      float64
    Low       float64
    Close     float64

    Volume    int64
    Amount    float64

    Timestamp int64 // unix time
}
```

---

## 4.2 Python DataFrame Schema

Must match Go exactly:

```python id="df_001"
columns = [
    "symbol",
    "timeframe",
    "open",
    "high",
    "low",
    "close",
    "volume",
    "amount",
    "timestamp"
]
```

---

## 4.3 Rule: No Extra Fields in Core Dataset

禁止：

* technical indicator columns in raw dataset
* mixed schema
* ad-hoc columns

---

# 5. Backtest Execution Standard

## 5.1 Event-driven Simulation Model

Python backtest must simulate Go behavior:

```text id="bt_001"
for each candle:
    update indicators
    check signals
    simulate entry/exit
```

---

## 5.2 Signal Interface

```python id="signal_001"
class Signal:
    symbol: str
    timestamp: int
    action: str  # BUY / SELL
    price: float
```

---

## 5.3 Portfolio Simulation

Required:

* position tracking
* entry price
* exit price
* PnL calculation
* fees simulation

---

# 6. Strategy Consistency Rule

## Critical Rule

> Python backtest logic MUST match Go production logic 1:1

---

### Forbidden

* Python using different MA calculation logic
* Python smoothing that Go does not use
* Different volume averaging methods

---

### Required

Shared logic definition:

* MA window
* Volume average window
* breakout condition
* support/resistance rule

---

# 7. Indicator Consistency Layer

Optional but recommended:

Create shared pseudo-spec:

```text id="spec_001"
MA20 = SUM(CLOSE, 20) / 20
VolumeAvg20 = SUM(VOLUME, 20) / 20
Breakout = CLOSE > RESISTANCE
```

Both Go and Python must implement exactly this.

---

# 8. Design Philosophy

* Go = real-time decision engine
* Python = truth validation engine
* MySQL = historical truth source
* Strategy correctness > performance

---

# 9. Key Principle

> Python discovers truth.
> Go executes truth.
> Data defines consistency.
