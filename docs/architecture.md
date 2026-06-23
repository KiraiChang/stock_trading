# 系統架構

## 整體資料流

```
FinMind API
    ↓ HTTP (每5分鐘 / 收盤後)
Go Market Data Service (Fetcher)
    ↓ BulkInsert
MySQL (candles table)
    ↓ GetLatestN(120根)
Indicator Engine (Go)
    ↓ Upsert
MySQL (indicator_snapshots) + Redis (hash, TTL 5min)
    ↓
Signal Engine
    ↓ Insert + LPush
MySQL (signals) + Redis (signal:queue)
    ↓ WebSocket Broadcast
Frontend Dashboard (Svelte)
```

---

## 模組關係

```
cmd/server/main.go
    ├── store (MySQL + Redis repos)
    ├── market (FinMindClient + Fetcher)
    ├── indicator (Engine)
    │       └── store.CandleRepo
    ├── signal (Engine)
    │       ├── indicator.Engine
    │       └── store.{CandleRepo, SignalRepo}
    ├── scheduler (cron jobs)
    │       ├── market.Fetcher
    │       └── signal.Engine
    └── api (Gin HTTP + WebSocket Hub)
            ├── handler.{Candle, Indicator, Signal, Watchlist}
            └── ws.Hub
```

---

## 技術選型理由

| 元件 | 選擇 | 理由 |
|------|------|------|
| Backend | Go | goroutine 適合多股票並行監控 |
| HTTP Router | Gin | 高效能、中介軟體齊全 |
| Database | MySQL | 結構化 OHLCV 資料、關聯查詢 |
| Cache | Redis | 低延遲熱資料存取 |
| WebSocket | gorilla/websocket | 穩定、廣泛使用 |
| Frontend | Vite + Svelte | 輕量、響應式、無 VDOM 開銷 |
| K線圖 | lightweight-charts | < 50KB、原生 Candlestick |

---

## 批次 vs 串流設計決策

Phase 1 採用**批次計算**：
- FinMind API 為 REST 輪詢，非即時 tick
- 5 分鐘 batch 計算足夠盤中監控需求
- 邏輯簡單、易於測試和驗證

Phase 2 換 Shioaji 後可改為 tick-level streaming rolling sum。
