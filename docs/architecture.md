# 系統架構

## 整體資料流

```
FinMind API
    ↓ HTTP（每 5 分鐘 / 收盤後）
Go Market Data Service（Fetcher）
    ↓ BulkInsert
SQLite / MySQL / PostgreSQL（candles table）
    ↓ GetLatestN（120 根）
Indicator Engine（Go）
    ↓ Upsert
DB（indicator_snapshots）+ Redis（hash, TTL 5 min）
    ↓
Signal Engine
    ↓ Insert
DB（signals）
    ↓ WebSocket Broadcast
Frontend（Svelte，由 Go backend 直接 serve）
```

---

## 模組關係

```
cmd/server/main.go
    ├── store（DB + Redis repos）
    ├── market（FinMindClient + Fetcher）
    ├── indicator（Engine）
    │       └── store.CandleRepo
    ├── signal（Engine）
    │       ├── indicator.Engine
    │       └── store.{CandleRepo, SignalRepo}
    ├── scheduler（cron jobs）
    │       ├── market.Fetcher
    │       └── signal.Engine
    ├── backtest（Manager，透過 Python 服務執行）
    └── api（Gin HTTP + WebSocket Hub + 前端靜態檔案）
            ├── handler.{Candle, Indicator, Signal, Watchlist, Backtest}
            └── ws.Hub
```

---

## Python 服務

Python 負責回測，獨立運行，透過共用 DB 與 Go 溝通。

```
Go（寫入 backtest_jobs）
    ↓ DB polling（Method A）或 HTTP（Method B）
Python Worker / HTTP Server
    ↓ 讀取 candles，執行 backtrader 回測
    ↓ 寫入 backtest_results + backtest_trades
Go API（讀取結果回傳給前端）
```

---

## 技術選型

| 元件 | 選擇 | 理由 |
|------|------|------|
| Backend | Go | goroutine 適合多股票並行監控 |
| HTTP Router | Gin | 高效能、中介軟體齊全 |
| Database | SQLite / MySQL / PostgreSQL | 三種環境皆支援，goose 自動 migration |
| Cache | Redis（選填） | 低延遲熱資料；addr 留空則停用 |
| WebSocket | gorilla/websocket | 穩定、廣泛使用 |
| Frontend | Vite + Svelte | 輕量、無 VDOM；build 後 embed 進 Go binary |
| K 線圖 | lightweight-charts | < 50KB、原生 Candlestick |
| Backtest | Python + backtrader | 策略研究與驗證 |

---

## 批次 vs 串流設計決策

Phase 1 採用**批次計算**：
- FinMind API 為 REST 輪詢，非即時 tick
- 5 分鐘 batch 計算足夠盤中監控需求
- 邏輯簡單、易於測試和驗證

Phase 2 換 Shioaji 後可改為 tick-level streaming rolling sum。

---

## 部署模式

### 開發環境

```
go run ./cmd/server      # SQLite，自動 migration
npm run dev              # Vite dev server（含 API proxy）
python worker.py         # Python worker（選填）
```

### 生產 / Docker

```
docker-compose up --build
```

- PostgreSQL + Redis 由 docker-compose 管理
- Go binary 內嵌前端靜態檔案（單一執行檔）
- Python worker 與 HTTP server 各自獨立 container
