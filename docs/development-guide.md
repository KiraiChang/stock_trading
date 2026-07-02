# 開發環境建立指南

## 環境需求

| 工具 | 版本 | 說明 |
|------|------|------|
| Go | 1.22+ | 後端 |
| Node.js | 20+ | 前端 build |
| Python | 3.8+ | 回測服務 |
| Docker + Compose | - | 生產環境（選填） |

SQLite 為開發預設，**不需要**安裝任何額外資料庫。

---

## 快速啟動（開發，SQLite）

### 1. 啟動後端

```bash
cd backend
go run ./cmd/server
```

- 首次啟動自動建立 `trading.db` 並套用所有 migration
- 監聽 `http://localhost:8080`

### 2. 啟動前端（開發模式）

```bash
cd frontend
npm install
npm run dev
```

- 監聽 `http://localhost:5173`
- API 請求自動 proxy 至 `localhost:8080`

### 3. 啟動 Python 回測服務（選填）

```bash
cd python
bash setup.sh          # 建立 venv、安裝依賴
bash start_worker.sh   # Method A：DB polling
bash start_server.sh   # Method B：FastAPI HTTP server（另開視窗）
```

---

## 切換到 MySQL 或 PostgreSQL

編輯 `backend/config.yaml`：

```yaml
database:
  driver: "postgres"   # 或 "mysql"
  dsn: "postgres://user:password@127.0.0.1:5432/trading?sslmode=disable"
  # MySQL DSN 範例：
  # dsn: "root:password@tcp(127.0.0.1:3306)/trading?parseTime=true&loc=Asia%2FTaipei"
```

編輯 `python/config.yaml`：

```yaml
database:
  driver: "postgres"
  dsn: "postgresql+psycopg2://user:password@127.0.0.1:5432/trading"
```

**Migration 不需要手動執行**，Go 啟動時 goose 會自動套用。

---

## 前端 Build（嵌入 Go Binary）

```bash
cd frontend
npm run build
# 輸出至 backend/internal/ui/dist/
```

之後重新 build Go：

```bash
cd backend
go build -o trading-backend ./cmd/server
./trading-backend
# 直接從 http://localhost:8080 服務前端，不需另起前端站台
```

---

## Docker Compose（生產環境）

```bash
docker-compose up --build
```

包含：PostgreSQL、Redis、Go backend、Python worker、Python HTTP server。

服務對應：

| 服務 | Port |
|------|------|
| Go backend（含前端） | 8080 |
| Python HTTP server | 8001 |
| PostgreSQL | 5432 |
| Redis | 6379 |

---

## 認證設定

### JWT Secret

開發環境預設值為 `change-me-in-production`，**生產環境必須更換**：

```bash
# 產生隨機 secret
openssl rand -hex 32
```

更新 `backend/config.yaml`：

```yaml
auth:
  jwt_secret: "your-random-secret-here"
```

或透過環境變數（Docker / CI 適用）：

```bash
AUTH_JWT_SECRET=your-random-secret-here
```

### 建立第一個使用者

**新帳號預設 `inactive`，需要手動啟用才能登入。**第一個管理員帳號需透過以下流程處理：

**Step 1. 註冊帳號**

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@trading.com","password":"secret123"}'
# → {"user_id":1,"email":"admin@trading.com","status":"inactive"}
```

**Step 2. 直接修改資料庫啟用第一個管理員**（因為此時還無法登入）

```bash
# SQLite（開發環境）
sqlite3 backend/trading.db "UPDATE users SET status='active' WHERE email='admin@trading.com';"
```

```sql
-- MySQL
UPDATE users SET status='active' WHERE email='admin@trading.com';

-- PostgreSQL
UPDATE users SET status='active' WHERE email='admin@trading.com';
```

**Step 3. 登入取得 token**

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@trading.com","password":"secret123"}'
# → {"token":"eyJhbGci...","expires_in":86400}
```

**Step 4. 後續使用者可透過管理頁面啟用**

登入後，在前端側欄點「⊙」進入使用者管理頁，或透過 API：

```bash
export TOKEN="eyJhbGci..."

# 啟用 id=2 的使用者
curl -X PATCH http://localhost:8080/api/v1/users/2/status \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"status":"active"}'
```

後續所有 API 請求帶入 token：

```bash
curl http://localhost:8080/api/v1/watchlist \
  -H "Authorization: Bearer $TOKEN"
```

Token 有效期 24 小時，過期後重新登入即可。

---

## 新增監控股票

```bash
curl -X POST http://localhost:8080/api/v1/watchlist \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"symbol":"2330","name":"台積電","sector":"半導體"}'
```

---

## Backfill 歷史資料

建議首次啟動後先 backfill 至少 120 天日 K（MA60 需要 60 根才能計算）：

```go
fetcher.BackfillHistory(ctx, symbols, 120)
```

也可以直接用前端「歷史資料回補」頁面（`/backfill`），勾選監控清單股票、
指定天數後送出，或呼叫 `POST /api/v1/market/backfill`（見 api-reference.md）。

---

## 個股分析

需要先 backfill 該股票至少 35 根日K，且 Python HTTP service 需已啟動
（`python.service_url` 已設定，見上方「啟動 Python 回測服務」）：

```bash
curl -X POST http://localhost:8080/api/v1/analysis \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"symbol":"2330","timeframe":"1d"}'
# → { "analysis": {...}, "levels": [...] }
```

過幾天後想確認支撐/壓力有沒有被突破、停損/停利有沒有被觸及，重新驗證
（可重複執行，每次都用最新 candles 重新計算，不用等排程）：

```bash
curl -X POST http://localhost:8080/api/v1/analysis/1/verify \
  -H "Authorization: Bearer $TOKEN"
```

也可以直接用前端「個股分析」頁面（`/analysis`），輸入代號即可，歷史紀錄
下方有「重新驗證」「刪除」按鈕。完整規格見 [stock-analysis.md](./stock-analysis.md)。

---

## 手動補算指標 / 評估訊號

排程只會處理監控清單裡的股票；如果某支股票（例如剛上市、還沒加進監控清單）
已經有 candles 但查 `/indicators/:symbol` 卻是 404，用這兩支端點手動補算，
兩者都**不要求在監控清單裡**，只要求 candles 至少 35 根：

```bash
# 手動算一次指標快照並寫入
curl -X POST "http://localhost:8080/api/v1/indicators/00981A/compute?timeframe=1d" \
  -H "Authorization: Bearer $TOKEN"

# 手動跑一次訊號判斷（內部會先呼叫指標計算）
curl -X POST "http://localhost:8080/api/v1/signals/00981A/evaluate?timeframe=1d" \
  -H "Authorization: Bearer $TOKEN"
# → 有觸發: {"signal": {...}}；沒觸發: {"signal": null, "message": "..."}
```

candles 不足 35 根時兩者都回 `422`，代表要先用「歷史資料回補」補更多天數。
前端「歷史資料回補」頁面（`/backfill`）下方的「手動計算指標」「手動評估訊號」
兩個區塊就是這兩支端點的 UI。

---

## 執行 Go 測試

`internal/signal/`（趨勢判斷/支撐壓力/突破訊號/Engine 整合）、
`internal/store/`（監控清單監聽上限）目前是唯二有測試的套件：

```bash
cd backend
go test ./internal/signal/... -v
go test ./internal/store/... -v
# 或直接跑全部（其他套件目前沒有測試檔，會顯示 [no test files]）
go test ./...
```

`internal/signal` 的整合測試（`engine_test.go`）會建立暫存 sqlite 檔案並
實際跑 migration，不需要額外設定；測試結束會自動清理暫存檔。

---

## 執行 Python 測試

`backtest/modular/` 的單元測試（支撐壓力/進場/停損/回測引擎/型別安全）：

```bash
cd python
.venv/Scripts/python.exe -m pip install pytest   # 或先 pip install -r requirements.txt
.venv/Scripts/python.exe -m pytest backtest/modular/tests -v
```

---

## 驗證 Fugle 即時行情（選填）

`fugle.enabled` 預設 `false`，接入前建議先用獨立工具驗證延遲與實際推播格式
（尤其盤中即時更新的訊息格式目前尚未確認，見 fugle-integration.md）：

```bash
cd backend
$env:FUGLE_ENABLED="true"; $env:FUGLE_API_KEY="<你的 API Key>"
go run ./cmd/fugle-check -symbol 2330 -duration 60s
```

盤中時段（09:00–13:30）執行才看得到即時推播；收盤後只會看到訂閱時的
`snapshot`（整包當日K棒）與 30 秒一次的 `heartbeat`。

---

## 常見問題

**PostgreSQL 連線失敗**：確認 `sslmode=disable`（本地開發無 TLS）。

**MySQL 連線失敗**：確認 DSN 中的時區 `loc=Asia%2FTaipei` 正確編碼。

**FinMind 回傳 401**：API Key 未設定或過期，至 `backend/config.yaml` 更新。

**Python 型別錯誤（'type' object is not subscriptable）**：Python 版本需 3.8+，`setup.sh` / `setup.ps1` 使用的 python 版本請確認。

**WebSocket 無法連線**：確認後端已啟動且防火牆允許 8080 port。

**API 回傳 401 Unauthorized**：Token 未帶入、已過期，或 JWT Secret 與簽發時不同。重新登入取得新 token。

**login 回傳 403 Forbidden**：帳號存在但 `status = inactive`，需要管理員在使用者管理頁或透過 `PATCH /users/:id/status` 啟用。

**register 回傳 409 Conflict**：該 email 已存在，直接用 login 取得 token 即可。

**`GET /indicators/:symbol` 回 404**：`indicator_snapshots` 沒有這支股票的資料，
通常是還沒加進監控清單（排程只處理監控清單）或 candles 不足 35 根。用
`POST /indicators/:symbol/compute` 手動補算，見上方「手動補算指標 / 評估訊號」。

**設定監聽（`PATCH /watchlist/:symbol/watch`）回 409**：即時監聽同時最多 3 檔
（`store.MaxWatchedSymbols`），需要先在前端監控清單頁面把其他股票的 ★ 取消
才能再設定新的。

**前端對某個數字欄位呼叫 `.toFixed()` 出現 `TypeError: ... is not a function`**：
後端有 struct 欄位直接用了標準庫的 `sql.NullFloat64`/`NullString`/`NullTime`，
序列化成 JSON 後會是 `{"Float64":123.45,"Valid":true}` 這種物件而不是單純的
數字/`null`。新增可空欄位務必用 `internal/store/null.go` 的
`store.NullFloat64`/`NullString`/`NullTime`，見 architecture.md「Nullable
欄位的 JSON 序列化」。
