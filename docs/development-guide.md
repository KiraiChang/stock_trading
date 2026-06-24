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

後端啟動後，先註冊帳號：

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@trading.com","password":"secret123"}'
```

登入取得 token：

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@trading.com","password":"secret123"}'
# → {"token":"eyJhbGci...","expires_in":86400}
```

後續所有 API 請求帶入 token：

```bash
export TOKEN="eyJhbGci..."

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

---

## 常見問題

**PostgreSQL 連線失敗**：確認 `sslmode=disable`（本地開發無 TLS）。

**MySQL 連線失敗**：確認 DSN 中的時區 `loc=Asia%2FTaipei` 正確編碼。

**FinMind 回傳 401**：API Key 未設定或過期，至 `backend/config.yaml` 更新。

**Python 型別錯誤（'type' object is not subscriptable）**：Python 版本需 3.8+，`setup.sh` / `setup.ps1` 使用的 python 版本請確認。

**WebSocket 無法連線**：確認後端已啟動且防火牆允許 8080 port。

**API 回傳 401 Unauthorized**：Token 未帶入、已過期，或 JWT Secret 與簽發時不同。重新登入取得新 token。

**register 回傳 409 Conflict**：該 email 已存在，直接用 login 取得 token 即可。
