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

### 1. 用 Docker 跑建置與測試

Backend：

```bash
docker run --rm \
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
  -v "$PWD/frontend:/app" \
  -w /app \
  node:20-alpine \
  sh -c "npm ci && npm run build"
```

Python：

```bash
docker run --rm \
  -v "$PWD/python:/app" \
  -w /app \
  python:3.11-slim \
  sh -c "pip install --no-cache-dir -r requirements.txt && pytest backtest/ -v"
```

### 2. 啟動 dev stack 做 smoke test

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

## 文件收斂規則

發現與實作不一致時：

- bug、矛盾結果、誤導行為、文件與實作不一致、已知限制：記到 `docs/issue.md`。
- 未來優化、功能擴充、重構、待規劃工作：記到 `docs/todo.md`。
- 已完成的 issue/todo 要移除；移除前，把值得長期保存的行為或設計寫回對應主題文件。
