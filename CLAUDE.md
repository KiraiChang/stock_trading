# CLAUDE.md

本文件提供 Claude Code（claude.ai/code）在此程式庫工作時的指引。
始終用繁體中文說明。

---

## Trading

台股即時技術分析與交易輔助系統

目標：

* 即時監控台股
* 計算技術指標（MA / RSI / MACD / VWAP / ATR）
* 偵測突破、跌破、爆量
* 產生交易訊號（人工決策）
* 不執行自動交易

---

# Architecture Principles

系統定位：

* Market Data Processing System
* Technical Analysis Engine
* Trading Signal Generator
* Human-in-the-loop Trading Assistant

非：

* HFT 系統
* Fully Automated Trading Bot
* Low-latency matching engine

---

# Technology Stack

## Backend

Language:

* Go

理由：

* goroutine 適合即時行情流
* WebSocket / streaming 處理強
* 適合多股票並行監控（1000~2000檔）
* 工程效率高、維護成本低

---

## Database (已實作)

### Primary Storage

* MySQL（已導入）

用途：

* Market Data（OHLCV）
* Technical Indicators Cache
* Signal History
* Watchlist Data

---

## Cache Layer

* Redis

用途：

* 即時行情快取
* 最新指標值
* Breakout / Signal queue

---

## Frontend

* React / Next.js

用途：

* 即時行情 dashboard
* Watchlist
* Signal alert panel
* K-line chart

---

## Market Data Source

### Phase 1 (已使用或可用)

* FinMind（歷史 / 分K / 日K）

### Phase 2

* Shioaji（即時行情 / Tick / 委買委賣）

---

# Market Data Model (MySQL)

## Table: candles

```go id="mysql_model_001"
type Candle struct {
    ID        uint64
    Symbol    string

    Timeframe string  // 1m / 5m / 1d

    Open      float64
    High      float64
    Low       float64
    Close     float64

    Volume    int64
    Amount    float64

    Timestamp time.Time
}
```

---

## Design Notes

* MySQL 已作為主要歷史資料庫
* 所有技術指標皆基於 candles 計算
* 不再依賴外部即時計算資料
* Redis 僅作為熱資料 cache

---

# Data Flow Architecture

```text id="flow_001"
Market Data Source
        ↓
Go Market Data Service
        ↓
MySQL (candles storage)
        ↓
Indicator Engine (Go)
        ↓
Redis (latest state cache)
        ↓
Signal Engine
        ↓
Notification / Dashboard
```

---

# Core Modules

## 1. Market Data Service

職責：

* 接收 FinMind / Shioaji data
* 轉換成 OHLCV candle
* 寫入 MySQL
* 更新 Redis 最新 K 線

---

## 2. Indicator Engine

所有指標皆基於 MySQL candles：

### Trend

* MA5 / MA10 / MA20 / MA60

### Momentum

* RSI
* MACD

### Volatility

* ATR
* Bollinger Bands

### Volume

* Volume MA
* Volume Spike

### Cost Proxy

* VWAP

---

## MA Calculation Rule

資料來源：

```text id="ma_source_001"
Close (from MySQL candles)
```

---

Rolling Optimization：

```text id="ma_opt_001"
MA = (PreviousSum - OldClose + NewClose) / N
```

避免 full scan query。

---

# Support & Resistance Engine

資料來源：

* MySQL OHLCV
* Volume distribution
* VWAP approximation

---

Outputs:

```go id="sr_model_001"
type Level struct {
    Price     float64
    Strength  float64
    Type      string // Support / Resistance
}
```

---

# Trend Detection

Market Structure:

* HH (Higher High)
* HL (Higher Low)
* LH (Lower High)
* LL (Lower Low)

---

Bullish:

* HH + HL

Bearish:

* LH + LL

---

# Breakout Detection

## Conditions

```text id="breakout_001"
1. Close > Resistance
2. Volume > AvgVolume(20)
3. Trend == Bullish
```

---

## Signal

```go id="signal_001"
if breakoutConfirmed {
    Signal = BUY
}
```

---

# Stop Loss Logic

## Conditions

```text id="stop_001"
1. Close < Support
2. Structure Broken (HL失效)
3. No recovery within 1~2 candles
```

---

Signal:

```go id="exit_001"
Signal = SELL
```

---

# Volume Spike

Definition:

```text id="volume_001"
Volume > 2 * MA(Volume, 20)
```

---

# Notification System

Channels:

* LINE Notify
* Telegram
* Email

---

Format:

```text id="notify_001"
[BREAKOUT ALERT]

Symbol: 8088
Price: 70.5

Volume: 2.3x

Signal: BUY
```

---

# Scanner Scope

## Phase 1

* Watchlist 50~200 stocks

---

## Phase 2

Full Market Scan:

* TWSE ~1000 stocks
* TPEx ~900 stocks
* Total ~1900 stocks

---

# System Philosophy

This system does NOT predict price.

It only evaluates:

* Trend state
* Breakout validity
* Support/Resistance integrity
* Volume confirmation
* Risk condition

---

# Non Goals

* High Frequency Trading
* Fully automated execution
* Market making
* Microsecond latency systems

---

# Roadmap

## Phase 1 (Current)

* MySQL-based market data storage
* Indicator engine
* Basic breakout detection

---

## Phase 2

* Real-time Shioaji integration
* Signal refinement (false breakout filter)
* Dashboard upgrade

---

## Phase 3

* Portfolio tracking
* Position management
* Strategy templates

---

## Phase 4

* Semi-auto execution (optional)
* Risk engine enhancement

---

# Key Design Shift

✔ MySQL is now the system of record
✔ Redis is only for hot cache
✔ Indicators computed from stored candles
✔ System is batch + streaming hybrid

---

# Core Insight

> This system is a data-driven decision support engine, not a trading robot.
