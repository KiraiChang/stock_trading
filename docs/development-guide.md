# 開發環境建立指南

## 環境需求

| 工具 | 版本 |
|------|------|
| Go | 1.22+ |
| Node.js | 20+ |
| MySQL | 8.0+ |
| Redis | 7.0+ |

---

## 1. 資料庫初始化

```bash
mysql -u root -p -e "CREATE DATABASE trading CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

mysql -u root -p trading < backend/migrations/001_create_candles.sql
mysql -u root -p trading < backend/migrations/002_create_indicators.sql
mysql -u root -p trading < backend/migrations/003_create_signals.sql
mysql -u root -p trading < backend/migrations/004_create_watchlists.sql
```

---

## 2. 設定 config.yaml

複製並編輯後端設定：

```bash
cp backend/config.yaml backend/config.local.yaml
```

修改 `backend/config.yaml`：

```yaml
mysql:
  dsn: "root:YOUR_PASSWORD@tcp(127.0.0.1:3306)/trading?parseTime=true&loc=Asia%2FTaipei"

finmind:
  api_key: "YOUR_FINMIND_API_KEY"
```

FinMind API Key 請至 [finmindtrade.com](https://finmindtrade.com) 註冊取得。

---

## 3. 啟動後端

```bash
cd backend
go mod tidy
go run ./cmd/server
```

後端預設監聽 `localhost:8080`。

---

## 4. 新增監控股票（範例）

```bash
curl -X POST http://localhost:8080/api/v1/watchlist \
  -H "Content-Type: application/json" \
  -d '{"symbol":"2330","name":"台積電","sector":"半導體"}'
```

---

## 5. Backfill 歷史資料

可透過直接呼叫 Fetcher 的 `BackfillHistory` 方法，或撰寫一個簡單的 CLI 工具。

---

## 6. 啟動前端

```bash
cd frontend
npm install
npm run dev
```

前端預設在 `http://localhost:5173`，API 請求透過 Vite proxy 轉發至後端。

---

## 常見問題

**MySQL 連線失敗**：確認 DSN 中的時區 `loc=Asia%2FTaipei` 正確編碼。

**FinMind 回傳 401**：API Key 未設定或過期，請重新取得。

**WebSocket 無法連線**：確認後端已啟動且防火牆允許 8080 port。
