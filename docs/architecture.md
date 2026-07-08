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
Signal Engine（含 chip_scores 強度加權）
    ↓ Insert
DB（signals）
    ↓ WebSocket Broadcast
Frontend（Svelte，由 Go backend 直接 serve）
```

收盤後 `daily_close` 會依序執行日 K 更新、日線訊號掃描、SR Zone 驗證與籌碼日結同步。
籌碼同步由 `chip.Syncer` 寫入 institutional/margin/broker raw tables 與 `chip_scores`，
失敗時只影響自己的 `job_runs` 紀錄，不會回滾 K 線或訊號結果。

Fugle 目前預設關閉（`fugle.enabled: false`），已完成 REST/WebSocket client 與
`cmd/fugle-check` 驗證工具，尚未接上 `Fetcher`/`scheduler` 的自動排程；詳見
[fugle-integration.md](./fugle-integration.md)。

FinMind 盤中分K排程也預設關閉（`finmind.intraday_enabled: false`）——該
dataset（`TaiwanStockKBar`）需要 Sponsor 級以上 token，帳號等級不足時
`runIntradayJob` 會直接跳過，不會每 5 分鐘對 FinMind 發出注定失敗的請求；
升級帳號後改成 `true` 即可恢復，見 [finmind-integration.md](./finmind-integration.md)。

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
    │       └── store.{CandleRepo, SignalRepo, ChipScoreRepo}
    ├── chip（Syncer + scoring helpers，三大法人、融資融券、券商分點、chip_scores）
    │       └── market.FinMindClient + store chip repos
    ├── scheduler（cron jobs，daily_close 收盤後接著跑 SR zone 驗證與 chip_daily_sync）
    │       ├── market.Fetcher
    │       ├── signal.Engine
    │       └── analysis.SRZoneVerifier
    ├── backtest（Manager，透過 Python 服務執行）
    ├── analysis（Client 呼叫 Python 計算：/analyze 與 /sr-zones 共用同一個
    │       Client；Verifier 與 SRZoneVerifier 都是純 Go 比對 candles 驗證，
    │       不呼叫 Python，見 sr-zone-scoring.md「十四」）
    └── api（Gin HTTP + WebSocket Hub + 前端靜態檔案）
            ├── middleware.Auth（JWT 驗證）
            ├── handler.Auth（register / login，新帳號預設 inactive）
            ├── handler.User（GET /users, PATCH /users/:id/status）
            ├── handler.{Candle, Indicator, Signal, Watchlist, Market, Backtest, Analysis}
            ├── handler.SRZone（store.SRZoneRepo，見 sr-zone-scoring.md）
            ├── handler.Chip（GET /chips/*, POST /chips/sync）
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
| Dashboard | `dashboard` | 監控清單（即時報價/RSI/量比/趨勢/訊號/★監聽切換）+ K線圖（可疊加 MA5/MA20/MA60，個別開關）+ 訊號面板 |
| 個股分析 | `analysis` | 輸入股票代號觸發分析（`POST /analysis`），顯示支撐/壓力/進場/停損/停利，可對歷史分析手動重新驗證或刪除 |
| 支撐/壓力機率分析 | `sr-zones` | 輸入股票代號觸發 SR Zone Scoring（`POST /sr-zones`），顯示機率模型算出的區間、機率、EV/RR、可拆解交易分數；另有「訓練/更新機率模型」區塊（`POST /sr-zones/train`）。詳見 [sr-zone-scoring.md](./sr-zone-scoring.md) |
| 持股操作 | `holdings` | 維護手中持股（代號、股數、持有成本），按下分析後用當次 SR Zone 快照產生並保存操作建議 |
| 歷史資料回補 | `backfill` | 勾選監控清單股票回補 K 棒（`POST /market/backfill`）；下方另有「手動計算指標」（`POST /indicators/:symbol/compute`）與「手動評估訊號」（`POST /signals/:symbol/evaluate`）兩個區塊，任意股票代號都可用 |
| 策略回測 | `backtest` | 送出回測任務（`POST /backtest`）、輪詢狀態、查看結果與逐筆交易 |
| 籌碼分析 | `chips` | 查詢籌碼摘要、歷史分數、券商分點排行，並可手動同步籌碼資料（`POST /chips/sync`） |
| 排程監控 | `scheduler` | 顯示 `pre_market`/`intraday`/`daily_close` 排程執行紀錄 |
| 使用者管理 | `users` | 啟用/停用帳號 |

Dashboard 的即時欄位以 REST 主動 hydrate（`/candles`、`/indicators`、`/signals`），
WebSocket 只在有新訊號時推播覆蓋，因為後端目前只會廣播 `signal` 事件。

WebSocket 訂閱不是整份監控清單，而是只對監控清單裡標記 `watched=true` 的
股票訂閱，且**同時最多 3 檔**（`store.MaxWatchedSymbols`，見
`PATCH /watchlist/:symbol/watch`）；`ws.Hub` 也有相同上限的防禦性檢查。
K線圖的 MA5/MA20/MA60 是前端用已載入的 candles 收盤價自行計算（rolling
window），不是另外呼叫 API，因為 `/indicators/:symbol` 只回傳最新一筆快照，
沒有整段歷史序列。

### 手動觸發端點（不限監控清單）

排程只會處理監控清單裡的股票；以下端點刻意設計成**任意股票代號**都能用，
用於補算/確認監控清單之外的股票（例如剛上市的 ETF、只是想先看看資料夠不夠）：

| 端點 | 說明 | 前提 |
|------|------|------|
| `POST /market/backfill` | 補歷史 K 棒 | 無 |
| `POST /indicators/:symbol/compute` | 手動算一次指標快照並寫入 | candles ≥ 35 根 |
| `POST /signals/:symbol/evaluate` | 手動跑一次訊號判斷（內部會先呼叫指標計算） | candles ≥ 35 根 |

三者都在「歷史資料回補」頁面有對應 UI，也可以直接呼叫 API（見
api-reference.md）。

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

模組化回測可選擇啟用 `use_chip_filter`，用 `chip_scores.total_score` 與
`chip_min_score` 門檻過濾進場訊號；legacy backtrader 策略收到該選項時會記 warning
並忽略，不中斷回測。

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

**SR Zone Scoring**（同樣是「Python 算、Go 存」，驗證階段跟個股分析一樣是
純 Go，見 sr-zone-scoring.md「十四」）：

```
Go POST Python /sr-zones（zone 建立 + 特徵計算 + 機率模型預測 + 分數推導）
    ↓
Go 寫入 stock_sr_zone_analyses + stock_sr_zones
    ↓（手動 POST /sr-zones/:id/verify，或 daily_close 排程每天自動驗證最近幾筆）
Go 讀 candles，純 Go 比對每個 zone 是否被突破
    ↓
更新 stock_sr_zones.status/broken_at/broken_price
```

訓練是獨立的非同步流程，不在上面這條同步路徑裡：

```
Go POST Python /sr-scoring/train（Go 端路由是 POST /sr-zones/train，立即回 202）
    ↓ 背景 goroutine，Python 端同步執行（可能耗時數十秒到數分鐘）
Python 訓練 hold_model + break_model，寫入 models/*.joblib
```

細節見 [sr-zone-scoring.md](./sr-zone-scoring.md)。

**籌碼分析**由 Go 端同步、計分與查詢，Python 只在模組化回測與 SR Zone v3 模型中讀取
已落地的 `chip_scores`：

```
manual/backfill API 或 daily_close
    ↓
chip.Syncer 從 FinMind 取得三大法人、融資融券、券商分點資料
    ↓
寫入 institutional_trades / margin_trades / broker_trades
    ↓
計算 chip_scores
    ↓
Signal Engine、Backtest、SR Zone Scoring 與前端 Chips 頁面讀取使用
```

`POST /api/v1/chips/sync` 建立 `chip_sync_jobs` 非同步任務；日結同步則使用
`job_runs.job_name=chip_daily_sync`，兩者的進度表不同。

**持股操作分析**重用 SR Zone Scoring，但保存成獨立快照：

```
使用者維護 holdings（symbol / shares / cost_price）
    ↓ POST /holdings/:id/analyze
Go 先查同 symbol/timeframe 既有 stock_sr_zone_analyses；有則重用最新快照，沒有才呼叫 Python /sr-zones 建立
    ↓
Go 依最新收盤價、持有成本、支撐/壓力 zone 產生操作建議
    ↓
寫入 holding_analyses（複製當下股數/成本，引用 sr_zone_analysis_id）
```

每次分析都新增一筆 `holding_analyses`，不覆蓋舊結果；`sr_zone_analysis_id` 可能引用既有
SR Zone 快照。後續修改持股股數或成本，也不會改變歷史分析快照。

### Nullable 欄位的 JSON 序列化

Go 的 `database/sql.NullFloat64` / `NullString` / `NullTime` 直接拿去
`json.Marshal` 會變成 `{"Float64":123.45,"Valid":true}` 這種內部結構，不是
單純的數字或 `null`，前端拿到後對它做數值運算（例如 `.toFixed()`）會直接
拋型別錯誤——`stock_analyses` 的多個可空欄位（`stop_loss_atr` 等）就中過這個
問題。**任何 API 會回傳的 struct，可空欄位一律用 `internal/store/null.go`
裡的 `store.NullFloat64` / `store.NullString` / `store.NullTime`**（內嵌
`sql.Null*` 保留 `Scan`/`Value` 給 sqlx，另外補上 `MarshalJSON`），不要直接用
標準庫的 `sql.Null*`。

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
docker network create proxy_net 2>/dev/null || true
docker-compose -f docker-compose.postgres.yml -f docker-compose.redis.yml -f docker-compose.yml up --build
```

- PostgreSQL 與 Redis 分別由 `docker-compose.postgres.yml` / `docker-compose.redis.yml` 管理；主 `docker-compose.yml` 負責 backend、python-worker、python-server，並假設 `trading-net` / `proxy_net` 已存在
- Go binary 內嵌前端靜態檔案（單一執行檔）
- Python worker 與 HTTP server 各自獨立 container
