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

- 首次啟動會依 `backend/config.yaml` 的 `database.dsn` 建立 SQLite DB 並套用所有 migration；目前預設路徑是 `../../output/stock_trading/trading.db`
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
docker network create proxy_net 2>/dev/null || true
docker-compose -f docker-compose.postgres.yml -f docker-compose.redis.yml -f docker-compose.yml up --build
```

包含：PostgreSQL、Redis、Go backend、Python worker、Python HTTP server。`docker-compose.yml`
本身假設 `trading-net` / `proxy_net` 已存在；上方 postgres compose 檔會建立
`trading-net`，`proxy_net` 需先存在或用上方命令建立。

服務對應：

| 服務 | Port |
|------|------|
| Go backend（含前端） | 8080 |
| Python HTTP server | 8001 |
| PostgreSQL | 5432 |
| Redis | 6379 |

app runtime log 會持久化在 repo root 的 `logs/`，避免 `deploy.sh` 或 compose redeploy 重新建立
container 後遺失：

| 服務 | 持久化路徑 |
|------|------------|
| Go backend | `logs/backend/` |
| Python HTTP server | `logs/python-server/` |
| Python worker | `logs/python-worker/` |

backend 每日寫入 `backend-YYYY-MM-DD.log`；Python 服務會寫入目前檔案
`python-server.log` / `python-worker.log`，並在每日輪替後留下日期後綴檔。app log 時間格式為 UTC
ISO 8601。保留天數可用 `LOG_RETENTION_DAYS` 調整，預設 14 天。

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
# SQLite（開發環境；路徑需對齊 backend/config.yaml 的 database.dsn）
sqlite3 ../../output/stock_trading/trading.db "UPDATE users SET status='active' WHERE email='admin@trading.com';"
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

## 籌碼資料同步

籌碼分析已接入前端「籌碼分析」頁面（`/chips`）與 API：

```bash
curl -X POST http://localhost:8080/api/v1/chips/sync \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"mode":"backfill","symbols":["2330","2454"],"dataTypes":["institutional","margin","broker","scores"]}'
# → { "job": { "job_id": "chip_...", "status": "pending", ... } }
```

`mode=manual` 且未指定日期時只同步今天；`mode=backfill` 且未指定 `from` 時，會用
`chip.sync.history_trading_days` 往回推。日結籌碼採集（`job_runs.job_name=chip_daily_sync`）
不與 15:00 `daily_close` 綁在一起，改由傍晚獨立 cron 觸發（設定 `chip.sync.cron`，預設 21:00），
因為 FinMind 法人／融資融券資料比日 K 晚發布，詳見 `docs/chip-analysis-design.md` §8。

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

## SR Zone Scoring（支撐/壓力機率分析）

跟個股分析是完全獨立的兩套系統（見 [sr-zone-scoring.md](./sr-zone-scoring.md)）。
除了 Python HTTP service 已啟動，**還需要先訓練過機率模型**，否則
`POST /sr-zones` 會失敗（fail-fast，不會靜默回傳中性機率）。分析前可以先
查詢模型狀態，不用等分析失敗才知道（前端頁面頂部也會顯示這個狀態）：

```bash
curl http://localhost:8080/api/v1/sr-zones/model-status \
  -H "Authorization: Bearer $TOKEN"
# → { "exists": false, "version": null, ... }  # 尚未訓練過
```

> `python/config.yaml` 的 `sr_scoring.model_path` 預設要跟目前的
> `MODEL_VERSION`（`model.py`）對得上——目前是 `models/sr_scoring_v4.joblib`。
> 如果本機還留著更早期重新設計前的 `sr_scoring_v1.joblib`，且 config 指
> 錯路徑，`/sr-zones` 會在預測時因為特徵數對不上而出現非預期的錯誤，不是
> 預期中的 503；確認 config 指向的檔名版本正確，或乾脆重新訓練一次覆蓋掉。

```bash
# 1. 先訓練模型（symbols 省略時自動用整個監控清單；非同步，立即回 202 + job_id）
curl -X POST http://localhost:8080/api/v1/sr-zones/train \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"symbols":["2330","2454"],"limit":1500}'
# → {"job_id":"sr_train_20260703_090000_000","status":"pending","message":"模型訓練已在背景啟動","symbols":2}

# 2. 用 job_id 輪詢進度，直到 status = done 或 failed
curl http://localhost:8080/api/v1/sr-zones/train-jobs/sr_train_20260703_090000_000 \
  -H "Authorization: Bearer $TOKEN"
# → { "job": { "status": "done", "rows": 128, "sources": 2, "metrics": {...}, "model_version": "v4", ... } }

# 3. 訓練完成後才能分析
curl -X POST http://localhost:8080/api/v1/sr-zones \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"symbol":"2330","timeframe":"1d","limit":250}'
# → { "analysis": {...global_*}, "zones": [...] }
```

`POST /sr-zones` 是同步分析，Go backend 對 Python `/sr-zones` 使用獨立逾時設定：
`python.sr_zones_timeout_sec`（預設 `120`，環境變數
`PYTHON_SR_ZONES_TIMEOUT_SEC`）。若資料量或 SHAP evidence 計算較重，可先延長
這個值；若仍然過慢，再調整 Python 端 `sr_scoring.evidence_max_zones` 或
`SR_SCORING_EVIDENCE_ENABLED=false` 讓 evidence 降級，評分本身仍會回應。

也可以直接用前端「支撐/壓力機率分析」頁面（`/sr-zones`），下方「訓練/更新
機率模型」區塊就是 `POST /sr-zones/train` 的 UI，觸發後會自動每 3 秒輪詢一次
狀態，完成/失敗都會顯示，下方也有最近幾次訓練紀錄。

過幾天後想確認每個 zone 有沒有被突破，重新驗證（可重複執行，15:00 的
`daily_close` 排程也會每天自動對最近幾筆分析呼叫一次，不用每次都手動觸發）：

```bash
curl -X POST http://localhost:8080/api/v1/sr-zones/1/verify \
  -H "Authorization: Bearer $TOKEN"
```

前端「支撐/壓力機率分析」頁面的分析結果右上角有「重新驗證」按鈕。

CLI 訓練（不透過 Go/HTTP，適合本地一次性訓練或除錯）：

```bash
cd python
.venv/Scripts/python.exe -m backtest.modular.sr_scoring.train \
  --symbols 2330,2454,0050 --timeframe 1d --limit 1500
# 輸出除了 metrics，還會印出資料集診斷摘要（每個 symbol 的 rows 數、
# hold/break positive rate、特徵為 0 比例等，見 sr-zone-scoring.md「四」）

# --split-method（預設 time）/ --calibration-method（預設 sigmoid）可調：
.venv/Scripts/python.exe -m backtest.modular.sr_scoring.train \
  --symbols 2330,2454 --split-method random --calibration-method none
```

---

## SR Zone / 分析輸出欄位開發注意事項

改 `sr_scoring`（scoring / decision_engine）或串接分析欄位到前後端時，容易踩到的
流程性問題，先照這幾點檢查再送出：

- **同名欄位跨 payload 要「同定義」**：像 `entry_relevance_score`、`zone_quality_score`
  這種同時出現在 `zones[]`（scoring 輸出）與 `decision_summary.*`（decision 輸出）的
  欄位，兩處必須是同一定義、同一值域。decision 層若要疊加市場事件之類的額外脈絡，
  用**內部變數**或**另取名**，對外一律回報 base 值——不要讓同名欄位在不同 payload
  帶不同數值，否則前端拿兩處對照會對不上。（`decision_engine` 的 `_entry_relevance_score`
  只回 base、`_entry_relevance_score_with_events` 才含事件修正且僅供內部 gating，就是
  這個原則的落實。）

- **Python → Go → TS 三端同步**：新增分析欄位（scoring/decision 產出的 JSON key）時，
  要同時補 `backend/internal/analysis/client.go` 的對應 struct 與
  `frontend/src/lib/api/srZones.ts` 的介面；新欄位用 `omitempty`（Go 指標）／optional
  （TS `?`）保持向後相容，舊資料/舊 client 不會壞。

- **enum 值比對用 `Enum.value`，不要硬編字面量**：decision/scoring 內比對 `tier`、
  `role`、`method` 等以 `str, Enum` 定義的欄位，用 `ZoneTier.TIER_x.value` 而非
  `"TIER_x"` 字面量；否則哪天調整 enum 值，字面量比對會**靜默失配**（例如
  `defense_lines` 的層別全變 `None`）而不報錯。

- **改門檻/權重前先確認測試涵蓋**：決策動作（BUY/BuySmall/EXIT）、市場事件偵測、
  regime 分層都有針對性測試；改 `_decision_action`、`_primary_zone_score`、
  `_detect_market_events` 的門檻或權重後，務必跑**兩套** pytest（見「執行 Python
  測試」）：`backtest/modular/tests` 與 `backtest/modular/sr_scoring/tests` 都要過。

- **tier / role 顯示標籤用共用模組**：SR Zone 的 tier / role 中文標籤（`TIER_LABEL_TEXT`、
  `ROLE_LABEL_TEXT`、`role_label`、`display_label`）集中在 `sr_scoring/labels.py`，`scoring.py`
  與 `decision_engine.py` 都從這裡 import，不要各自再定義一份，否則調整標籤（例如 tier 文案）
  時容易漏改一邊造成 drift。

- **Go 端 map[string]any 取值 helper 刻意分散、不強求收斂**：「從 `map[string]any` 依 path
  取型別值」的 helper 在 `internal/analysis/client.go`（回傳原生指標型別）、`internal/store`
  （回傳 `store.Null*`）、`internal/api/handler/sr_zones.go`（`store.RawJSON` I/O）三處各自實作。
  已評估過收斂為單一 util，結論是**不做**：三者回傳型別與所在 package 需求各異，真正重複的只有
  幾行 map-path walk，抽成單一 util 反而要引入跨 package 依賴、並把 handler 的 RawJSON 反序列化
  硬歸成「map 取值」。新增取值需求時就近沿用該 package 既有 helper 即可。

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

目前有測試的套件：`internal/signal/`（趨勢判斷/支撐壓力/突破訊號/Engine
整合）、`internal/store/`（監控清單監聽上限、SR Zone Repo 的 Create/Get/
List/Delete round-trip）、`internal/analysis/`（`Client.Analyze`/
`ScoreZones`/`TrainModel` 對 Python HTTP service 的請求/回應解析，用
`httptest.NewServer` 模擬，不需要真的啟動 Python）：

```bash
cd backend
go test ./internal/signal/... -v
go test ./internal/store/... -v
go test ./internal/analysis/... -v
# 或直接跑全部（其他套件目前沒有測試檔，會顯示 [no test files]）
go test ./...
```

`internal/signal` 的整合測試（`engine_test.go`）會建立暫存 sqlite 檔案並
實際跑 migration，不需要額外設定；測試結束會自動清理暫存檔。

---

## 執行 Python 測試

`backtest/modular/tests/` 是支撐壓力/進場/停損/回測引擎/型別安全的單元測試；
`backtest/modular/sr_scoring/tests/` 是 SR Zone Scoring 的單元測試（zone
建立/特徵工程/labeling/dataset/model/scoring），**獨立的測試套件、獨立的
conftest**（刻意不跨套件 import，理由見 sr-zone-scoring.md 開頭），兩者都要
跑：

```bash
cd python
.venv/Scripts/python.exe -m pip install pytest   # 或先 pip install -r requirements.txt
.venv/Scripts/python.exe -m pytest backtest/modular/tests -v
.venv/Scripts/python.exe -m pytest backtest/modular/sr_scoring/tests -v
# 或一次跑全部（backtest/ 底下所有測試）：
.venv/Scripts/python.exe -m pytest backtest/ -v
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

**前端對某個物件欄位（例如 `trading_score_breakdown`）取子欄位是 `undefined`**：
後端把一段 JSON 內容存成 Go `string`/`json.RawMessage`（`[]byte`）欄位直接
`json.Marshal`，會逃逸成一個 JSON 字串（`"{\"foo\":1}"`）或（在 PostgreSQL/
pgx 下）觸發 `sql: Scan error ... storing driver.Value type string into type
*json.RawMessage`（pgx 把 TEXT 欄位讀成 `string`，`database/sql` 不會自動轉成
`[]byte`-based 型別）。修法：用 `internal/store/null.go` 的 `store.RawJSON`
（底層是 `string`，`MarshalJSON` 把內容原樣嵌入回應），不要用
`json.RawMessage`/`[]byte` 存這種「DB 裡是 TEXT、API 要回傳巢狀 JSON
object」的欄位——這個錯誤只在 PostgreSQL 才會出現，SQLite/MySQL 不會報錯，
本機用 SQLite 開發測不出來，上 VPS（PostgreSQL）才會炸。

**`POST /sr-zones` 回 `502 Bad Gateway`**：Python HTTP service 沒開，或
`python.service_url`/`PYTHON_SERVICE_URL` 未設定。若 Python service 有回應但
內容是模型相關錯誤（`RuntimeError`／`503`），代表機率模型還沒訓練過，先呼叫
`POST /sr-zones/train`（或 CLI `python -m backtest.modular.sr_scoring.train`）
訓練完成後再重試，見上方「SR Zone Scoring」一節。

**`POST /sr-zones` 回 `504 Gateway Timeout`**：Go 已連到 Python，但 Python
分析時間超過 `python.sr_zones_timeout_sec`。先視環境延長
`PYTHON_SR_ZONES_TIMEOUT_SEC`；若仍常發生，降低 `SR_SCORING_EVIDENCE_MAX_ZONES`
或設定 `SR_SCORING_EVIDENCE_ENABLED=false`，讓 SHAP evidence 降級。
