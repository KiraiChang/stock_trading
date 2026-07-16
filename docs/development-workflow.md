# Development Workflow

這份文件是本專案共同的開發與驗收工作方式。`AGENTS.md` 與 `CLAUDE.md` 都應引用這裡，
避免不同 agent 使用不同流程。

## Docker 環境區隔

開發驗收一律使用 dev 專用 compose，不要用 live/deploy 使用的 compose project。

| 用途 | Compose | Project / resources | Port |
|------|---------|---------------------|------|
| 開發驗收 | `docker-compose.dev.yml` | `stock_trading_dev`、`stock_trading_dev_net`、`stock_trading_dev_*` volumes | Backend `18080`、Python `18001`、Postgres `15432`、Redis `16379` |
| live / 部署 | `docker-compose.yml` + `deploy.sh` 或正式部署流程 | live project、live network、live volumes | 依部署設定 |

規則：

- 驗收開發成果時只跑 `docker-compose.dev.yml`。
- 不要對 live project 執行 `docker compose down -v`、migration 測試、資料清空或測試資料匯入。
- 需要重置驗收資料時，只能對 dev compose 執行 `down -v`。
- dev compose 已使用不同 host ports，可與 live 同機並存。

## Docker 驗收流程

從 repo root 執行。

單次驗收用的 `docker run` 預設加上資源限制，避免測試或 build 佔滿主機資源：

| 參數 | 限制 |
|------|------|
| `--cpus="0.5"` | 最多使用半顆 CPU 的運算時間 |
| `--memory="512m"` | 實體記憶體上限 512MB |
| `--memory-swap="768m"` | 記憶體加 swap 總上限 768MB |
| `--pids-limit=200` | 程序及執行緒數量上限 200 |

### 1. 用 Docker 跑建置與測試

Backend：

```bash
docker run --rm \
  --cpus="0.5" \
  --memory="512m" \
  --memory-swap="768m" \
  --pids-limit=200 \
  -v "$PWD/backend:/app" \
  -w /app \
  -e GOMODCACHE=/tmp/gomod \
  -e GOCACHE=/tmp/gocache \
  golang:1.25-alpine \
  go test ./...
```

Frontend：

```bash
docker run --rm \
  --cpus="0.5" \
  --memory="512m" \
  --memory-swap="768m" \
  --pids-limit=200 \
  -v "$PWD/frontend:/app" \
  -w /app \
  node:20-alpine \
  sh -c "npm ci && npm run build"
```

Python：

```bash
docker run --rm \
  --cpus="0.5" \
  --memory="512m" \
  --memory-swap="768m" \
  --pids-limit=200 \
  -v "$PWD/python:/app" \
  -w /app \
  python:3.11-slim \
  sh -c "pip install --no-cache-dir -r requirements.txt && pytest backtest/ -v"
```

### 2. 啟動 dev stack 做 smoke test

`docker-compose.dev.yml` 已對 dev stack 服務套用同樣的資源限制：
每個 container 預設最多 `0.5` CPU、`512m` 實體記憶體、`768m` memory+swap、`200` 個
process/thread。

```bash
docker compose -f docker-compose.dev.yml up --build -d
docker compose -f docker-compose.dev.yml ps
```

健康檢查：

```bash
curl http://localhost:18080/health
curl http://localhost:18001/health
```

查看 log：

```bash
docker compose -f docker-compose.dev.yml logs --tail=200 backend
docker compose -f docker-compose.dev.yml logs --tail=200 python-server
```

dev stack 也會把 app runtime log 寫到 repo root 的 `logs/dev/`，避免 container 重新建立後只剩
Docker stdout 可查：

| 服務 | 持久化路徑 |
|------|------------|
| backend | `logs/dev/backend/` |
| python-server | `logs/dev/python-server/` |
| python-worker | `logs/dev/python-worker/` |

backend 會寫每日檔案 `backend-YYYY-MM-DD.log`；Python 服務會寫目前檔案
`python-server.log` / `python-worker.log`，並在每日輪替後保留日期後綴檔案。app log 時間一律使用
UTC ISO 8601，保留天數由 `LOG_RETENTION_DAYS` 控制，預設 14 天。

停止 dev stack：

```bash
docker compose -f docker-compose.dev.yml down
```

清空 dev 驗收資料：

```bash
docker compose -f docker-compose.dev.yml down -v
```

## 開發完成標準

完成程式修改後，至少要做：

- 受影響 runtime 的 Docker 測試或 build。
- 若有 migration、API、跨服務整合、排程或 Python/Go 互動，啟動 dev stack 做 smoke test。
- 若有前端畫面變更，跑 frontend Docker build，並在 dev stack 或本地 dev server 驗證畫面。
- 若因環境、網路或外部 token 無法執行某項驗證，最後回報要明確寫出未執行項目與原因。

## 開發慣例（品質守則）

這些是從實際 code review 累積的品質守則，補在「開發完成標準」之外，針對容易長期潛藏、
不會在單次功能測試中浮現的缺陷型態。每條都附上實際踩過的案例，方便對照。

### 1. 測試對「規格」斷言，不要對「當下輸出」斷言

- **原則**：凡是文件（主題文件／spec）有明確公式或契約的行為，測試的期望值要依規格獨立算出，
  而不是把程式當下的輸出貼回斷言。
- **為什麼**：`chip_summary.effective_score` 未依 coverage 降權的缺陷長期存在，正是因為測試直接
  斷言了錯誤輸出（滿覆蓋斷言 `== 42.5`，即未降權的 `total_score`），另一筆 fixture 又剛好讓兩種
  公式相等而遮蔽分歧——測試反而幫 bug 背書。
- **具體做法**：
  - fixture 刻意挑「會讓錯誤實作失敗」的輸入（例如讓 `total_score ≠ raw_score * coverage`）。
  - 期望值用規格公式手算或獨立推導，並在測試留註解說明來源。
  - review 時問一句：這個期望值來自規格，還是來自現有輸出？

### 2. 對外文件化的欄位要有契約測試

- **原則**：只要欄位寫進對外契約（例如 `sr-zone-scoring.md` 的 `decision_summary` /
  `chip_summary`），就要有測試守住「欄位存在，且宣稱非 null 的確實有值」。
- **為什麼**：本次 review 靠人工才發現 `price_path.blocking_zone.zone_id` / `timeframe` 恆為 null、
  部分「拆分」欄位其實是別名、`final_entry_permission` 的 `BUY` 端到端不可達。`sr-zone-scoring.md`
  本就要求「新分析缺欄位應由 Python 單元測試攔下」，但覆蓋不完整，落差只能靠人工發現。
- **具體做法**：
  - 對代表性輸入做 snapshot／欄位集合斷言，契約偏移時 CI 先擋。
  - 對「文件宣稱一定有值」的欄位加非 null 斷言；若目前刻意恆 null（如 `zone_id`），文件要標明
    現況、測試斷言其為 null，讓文件與測試兩邊一致。

### 3. 新增 `decision_summary` 欄位，把「前端消費」納入完成定義

- **原則**：Python 端新增／拆分 `decision_summary` 欄位時，要一併決定前端如何呈現，不能只加
  Python 輸出與 TS 型別就當完成。
- **為什麼**：`final_entry_permission`、`rr_context`、`nearest_support/resistance_zone` 等一度只在
  Python 產出、TS 宣告型別，但 `SRZones.svelte` 未渲染、舊單一欄位仍顯示造成誤導（該批欄位已於
  `SRZones.svelte` 補上呈現）。Python↔Go↔TS 五層契約有寫，但沒有機制確保新型別真的被消費，這條
  慣例就是要補上這個缺口。
- **具體做法**：
  - plan/PR 明確標記每個新欄位的前端處置：「本次接線」或「顯式延後到 T-xxx」。
  - 顯示新拆分欄位時，同步移除或標註被取代的舊單一欄位，避免殘留誤導。

### 4. 模組 import 不得有連線等副作用，測試要能獨立啟動

- **原則**：import 一個模組不應該就去連 DB／外部服務；連線要延後到真正使用時（lazy），
  讓純單元測試不依賴外部環境。
- **為什麼**：`db.py` 原本在 import 時就 `engine.connect()` 並在失敗時 `raise`，任何 import 到
  `db`（或間接透過 `scoring` 等）的模組在連不到 DB 時都會在收集階段整包失敗，害 §1 的
  `pytest backtest/` 在乾淨容器跑不起來。現已改為 lazy：連線健康檢查抽成 `db.check_connection()`，
  由服務啟動路徑（`http_server` / `worker` / `train` CLI）明確呼叫，import `db` 不再有連線副作用，
  §1 的指令因此不需要任何 DB 環境變數覆寫即可跑。
- **具體做法**：
  - 需要啟動時 fail-fast 的服務／CLI，在進入點呼叫 `db.check_connection()`，不要靠 import 副作用。
  - 新寫的模組比照辦理：建立 engine／client 可以在 module level，但實際連線要延後到使用或明確的
    啟動檢查，別放在 import 時執行。

## 文件收斂規則

發現與實作不一致時：

- bug、矛盾結果、誤導行為、文件與實作不一致、已知限制：記到 `docs/issue.md`。
- 未來優化、功能擴充、重構、待規劃工作：記到 `docs/todo.md`。
- 已完成的 issue/todo 要移除；移除前，把值得長期保存的行為或設計寫回對應主題文件。
