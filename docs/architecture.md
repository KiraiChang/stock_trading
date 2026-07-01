# 系統架構

## 整體資料流

```
FinMind API（歷史日K / 分K）      Fugle MarketData API（盤中即時，選填，見 fugle-integration.md）
    ↓ HTTP（每 5 分鐘 / 收盤後）        ↓ REST（quote/candles）+ WebSocket（candles channel）
Go Market Data Service（Fetcher）←──────┘
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

Fugle 目前預設關閉（`fugle.enabled: false`），已完成 REST/WebSocket client 與
`cmd/fugle-check` 驗證工具，尚未接上 `Fetcher`/`scheduler` 的自動排程；詳見
[fugle-integration.md](./fugle-integration.md)。

---

## 模組關係

```
cmd/server/main.go
    ├── store（DB + Redis repos，含 UserRepo）
    ├── market（FinMindClient + Fetcher，選填 FugleQuoteClient / FugleStreamClient）
    ├── indicator（Engine）
    │       └── store.CandleRepo
    ├── signal（Engine）
    │       ├── indicator.Engine
    │       └── store.{CandleRepo, SignalRepo}
    ├── scheduler（cron jobs）
    │       ├── market.Fetcher
    │       └── signal.Engine
    ├── backtest（Manager，透過 Python 服務執行）
    ├── analysis（Client 呼叫 Python 計算，Verifier 純 Go 比對 candles 驗證）
    └── api（Gin HTTP + WebSocket Hub + 前端靜態檔案）
            ├── middleware.Auth（JWT 驗證）
            ├── handler.Auth（register / login，新帳號預設 inactive）
            ├── handler.User（GET /users, PATCH /users/:id/status）
            ├── handler.{Candle, Indicator, Signal, Watchlist, Market, Backtest, Analysis}
            └── ws.Hub
```

## 認證架構

```
Client
    ↓ POST /api/v1/auth/login（email + password）
Auth Handler
    ↓ bcrypt.CompareHashAndPassword
    ↓ 檢查 user.status == "active"（否則 → 403 Forbidden）
    ↓ jwt.NewWithClaims（HS256，TTL 24h）
    ← 返回 JWT token
Client
    ↓ Authorization: Bearer <token>（每個受保護的請求）
middleware.Auth
    ↓ jwt.ParseWithClaims → 驗簽 + 驗期限
    ↓ 注入 user_id / email 至 Gin context
Handler（受保護）
```

所有 `/api/v1/*` 路由除 `/auth/register` 和 `/auth/login` 外，皆需通過 JWT 驗證。

### 使用者狀態機

```
register → inactive ──→（管理員 PATCH /users/:id/status）──→ active ──→ 可登入
                                                                  ↓
                                                              inactive（停用）
```

- 新帳號一律 `inactive`，防止未授權使用者自行進入系統
- 第一個管理員需直接修改資料庫啟用（見開發指南）
- 後續帳號可由已登入的管理員透過前端「使用者管理」頁或 API 啟用

---

## 前端頁面

Svelte 單頁應用（`frontend/src/routes/`），登入後由 Sidebar 切換：

| 頁面 | Route | 說明 |
|------|-------|------|
| Dashboard | `dashboard` | 監控清單（即時報價/RSI/量比/趨勢/訊號）+ K線圖 + 訊號面板 |
| 個股分析 | `analysis` | 輸入股票代號觸發分析（`POST /analysis`），顯示支撐/壓力/進場/停損/停利，並可對歷史分析手動重新驗證 |
| 歷史資料回補 | `backfill` | 勾選監控清單股票，呼叫 `POST /market/backfill` |
| 策略回測 | `backtest` | 送出回測任務（`POST /backtest`）、輪詢狀態、查看結果與逐筆交易 |
| 排程監控 | `scheduler` | 顯示 `pre_market`/`intraday`/`daily_close` 排程執行紀錄 |
| 使用者管理 | `users` | 啟用/停用帳號 |

Dashboard 的即時欄位以 REST 主動 hydrate（`/candles`、`/indicators`、`/signals`），
WebSocket 只在有新訊號時推播覆蓋，因為後端目前只會廣播 `signal` 事件。

---

## Python 服務

Python 負責回測，獨立運行，透過共用 DB 與 Go 溝通。

```
Go（寫入 backtest_jobs）
    ↓ DB polling（Method A）或 HTTP（Method B）
Python Worker / HTTP Server
    ↓ backtest/engine.py 依 strategy 名稱分派：
    │   - 命中 backtest/modular/strategy.py 的 STRATEGY_PRESETS → 走純 pandas/numpy
    │     的模組化引擎（見 backtest-modular-strategy.md）
    │   - 否則走既有 backtrader 引擎（STRATEGY_MAP）
    ↓ 寫入 backtest_results + backtest_trades（兩條路徑輸出格式一致）
Go API（讀取結果回傳給前端）
```

**個股分析**（`analysis` package）跟回測共用「Python 算、Go 存」的分工，但驗證
階段反過來——比對已存的價位跟 candles 大小，不需要重跑 Python 的策略邏輯：

```
Go POST Python /analyze（現況計算：支撐/壓力/進場/停損/停利）
    ↓
Go 寫入 stock_analyses + stock_analysis_levels
    ↓（之後手動觸發，可重複執行）
Go 讀 candles，純 Go 比對支撐/壓力是否突破、停損/停利是否觸及
    ↓
更新 stock_analyses.trade_verification + stock_analysis_levels.status
```

細節見 [stock-analysis.md](./stock-analysis.md)。

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
| Backtest | Python + backtrader，另有純 pandas/numpy 模組化引擎 | 策略研究與驗證；模組化引擎可獨立替換 S/R、進場、停損元件（Strategy Pattern） |
| Auth | JWT（HS256）+ bcrypt | 無狀態 token，密碼安全雜湊 |

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
