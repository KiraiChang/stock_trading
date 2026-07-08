# CLAUDE.md

本文件提供 Claude Code（claude.ai/code）在此程式庫工作時的指引。
始終用繁體中文說明。

共用開發與 Docker 驗收流程以 `docs/development-workflow.md` 為準。驗收開發成果時要使用
`docker-compose.dev.yml` 的 dev project，不能使用 live/deploy 的 compose project
做測試資料、migration 驗證或清空資料。

## 協作流程

收到需求後，先回饋並整理理解到的需求內容，等待使用者確認。
需求確認前，不要開始瀏覽程式碼、查閱文件、規劃或執行任何變更。

需求確認後，才開始瀏覽必要檔案並提出執行計畫。
計畫需等待使用者再次確認；計畫確認完成後，才開始實際執行、修改檔案、執行測試或啟動服務。

整理發現或規劃事項時，不要預設新增獨立文件。依性質分流：

* bug、矛盾結果、誤導行為、文件與實作不一致、已知限制，記錄到 `docs/issue.md`。
* 未來優化、功能擴充、重構、待規劃工作，記錄到 `docs/todo.md`。
* 需要長期保存的設計、操作或架構說明，補到 `docs/` 內既有相對應主題文件。
* 只有在使用者明確要求新增獨立文件，或沒有任何既有主題文件適合承接時，才建立新的 docs 文件。

項目完成後要收斂清單，不要讓 `docs/issue.md` / `docs/todo.md` 累積已結案項目：

* `docs/issue.md` 的項目修復完成、或 `docs/todo.md` 的項目實作完成後，把該筆整筆從清單移除。
* 移除前，先把需要長期保留的行為、設計或限制說明，更新到 `docs/` 內對應主題文件（例如 `stock-analysis.md`、`sr-zone-scoring.md`）成為「現況說明」；沒有對應文件時才補在最合適的既有文件。移除後同步修掉其他文件指向該筆的交叉引用，避免斷鏈。
* 兩份清單只保留仍待處理的項目（`issue.md`：待修復／修復中，或刻意保留且尚未文件化的已知限制；`todo.md`：待規劃／規劃中／進行中）。若一筆分類錯置（例如功能擴充被寫進 issue.md），移到正確的清單而不是留著。

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
