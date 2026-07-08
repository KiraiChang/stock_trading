# stock_trading

台股監控、訊號、回測與支撐壓力分析系統。專案由三個 runtime 組成：

- `backend/`：Go + Gin API，負責認證、資料庫、排程、訊號、分析持久化與內嵌前端。
- `frontend/`：Svelte/Vite SPA，提供監控清單、K 線、回測、排程、個股分析、SR Zone 與籌碼頁面。
- `python/`：回測、個股分析與 SR Zone Scoring 的計算服務，可用 worker 輪詢或 FastAPI HTTP server。

## Quick Start

```bash
cd backend
go run ./cmd/server
```

```bash
cd frontend
npm install
npm run dev
```

```bash
cd python
bash setup.sh
bash start_worker.sh
bash start_server.sh
```

開發預設使用 SQLite，資料庫路徑由 `backend/config.yaml` 與 `python/config.yaml`
設定，目前預設為 `../../output/stock_trading/trading.db`。Python HTTP service
只有在 `backend/config.yaml` 的 `python.service_url` 有設定時，Go 才會主動呼叫。

## Main Features

- JWT 認證與使用者啟用/停用。
- Watchlist、K 線 backfill、指標快照與突破/跌破/爆量訊號。
- 收盤排程：日 K 更新、訊號掃描、SR Zone 驗證與籌碼日結同步。
- Python 回測 job，支援 legacy backtrader 與模組化 pandas/numpy 策略。
- 個股支撐/壓力分析與 SR Zone Scoring 機率模型。
- 籌碼分析：三大法人、融資融券、券商分點、籌碼分數與同步 job。
- Fugle 即時行情 client 與驗證工具，預設關閉，尚未完整接入自動排程。

## Useful Docs

- 開發與啟動：[docs/development-guide.md](docs/development-guide.md)
- 系統架構：[docs/architecture.md](docs/architecture.md)
- API：[docs/api-reference.md](docs/api-reference.md)
- 資料庫 schema：[docs/database-schema.md](docs/database-schema.md)
- SR Zone Scoring：[docs/sr-zone-scoring.md](docs/sr-zone-scoring.md)
- 籌碼分析：[docs/chip-analysis-design.md](docs/chip-analysis-design.md)
- 已知問題：[docs/issue.md](docs/issue.md)
- 待辦與未來改善：[docs/todo.md](docs/todo.md)
