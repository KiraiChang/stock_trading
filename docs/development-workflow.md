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

## 測試腳本優先

建置與測試一律走各 runtime 的腳本，不要臨時手打 `docker run`：

1. 要跑建置或測試時，先看該 runtime 的 `scripts/` 有沒有現成腳本；**有就用腳本**。
2. 現有腳本涵蓋不到當次需求時（新的檢查類型、新 runtime、需要固定的新參數），
   **先提出需求與使用者確認，把行為補進腳本，再用腳本執行**，不要用一次性指令繞過。
3. 一次性指令只用於診斷（例如進 container 看狀態），不作為驗收依據。

Dev stack smoke 也走 repo script：`scripts/smoke-dev.sh` 會啟動 isolated dev compose、
等待 backend 與 python-server health check 通過，失敗時自動印出服務狀態與核心 log。

理由：手打指令會漂移，本專案已經因此踩過三個坑——以 root 執行留下 root-owned 檔案
（`backend/server` 曾被誤 commit、`backend/internal/ui/dist` 一度無法被覆寫）、
記憶體上限不足導致 Go build OOM、frontend 只掛 `frontend/` 導致 build 產物寫進
container 內憑空消失且不會報錯。腳本是這些約束的唯一真實來源。

## Docker 驗收流程

從 repo root 執行。

### 1. 用測試腳本跑建置與測試

| Runtime | 腳本 | 預設行為 |
|---------|------|----------|
| Backend (Go) | `backend/scripts/test.sh [packages...]` | `go vet` → `go test` → `go build`（全部套件為 `./...`） |
| Python | `python/scripts/test.sh [pytest 參數/路徑]` | 用 `python/Dockerfile` 建測試 image，跑 `pytest backtest/ tests/` |
| Frontend (Svelte) | `frontend/scripts/test.sh [--install]` | `svelte-check`（型別）→ `vitest run`（單元）→ `vite build`，任一失敗即中止 |

```bash
backend/scripts/test.sh                          # 全部 Go 套件
backend/scripts/test.sh ./internal/market/...    # 只驗單一套件
TEST_FLAGS="-count=1 -v" backend/scripts/test.sh ./internal/market/...

python/scripts/test.sh                                   # backtest/ 與 tests/
python/scripts/test.sh backtest/modular/sr_scoring/tests # 指定目錄
python/scripts/test.sh -k event_engine backtest/         # 直接帶 pytest 參數

frontend/scripts/test.sh            # 沿用現有 node_modules
frontend/scripts/test.sh --install  # 先 npm ci（node_modules 不存在時會自動加上）
```

三支腳本共同保證：

- `--user "$(id -u):$(id -g)"`：以本機 uid/gid 執行，container 產出的檔案不會是 root 所有。
- Go build 產物寫到 container 內 `/tmp`、pytest 關閉 `__pycache__` 與 `.pytest_cache`，
  不在 repo 留下編譯／測試殘渣。
- build / module / npm 快取放在 repo 外的 `~/.cache/stock_trading/`（可用 `CACHE_DIR=` 覆寫），
  跨次重用且不會被誤加進版控。

資源限制（預設值，可用環境變數覆寫）：

| 項目 | Backend | Python | Frontend |
|------|---------|--------|----------|
| `MEM`（記憶體上限） | `1800m` | `1024m` | `1024m` |
| `CPUS` | `1` | `1` | `1` |
| `--pids-limit` | 200 | 200 | 200 |
| image 覆寫 | `GO_IMAGE` | `PY_IMAGE` | `NODE_IMAGE` |

Go 另外固定 `GOMAXPROCS=1` + `GOFLAGS=-p=1`：本機只有 2GiB RAM，平行編譯會 OOM，
必須序列編譯；記憶體上限也因此不能沿用其他 runtime 的 512m。

Frontend 注意事項：`vite.config.ts` 的 `outDir` 是 `backend/internal/ui/dist`
（Go embed 使用、且有進版控），所以腳本掛載的是 **repo root** 而非 `frontend/`。
跑完 `git status` 出現 dist 差異屬正常，要不要保留該次產物由當次工作決定。

Frontend 測試框架（三層，T-033 導入）：

- **型別**：`svelte-check`（含 `.svelte` 內 TS），對應 `npm run check`。`tsconfig.json` 的
  `strict: true` 從此被真正執行；`src/vite-env.d.ts` 補上 `vite/client` 型別讓 `import.meta.env`
  可用。
- **單元 / 元件**：`vitest` + `@testing-library/svelte`（v4，對應 Svelte 4）+ `jsdom`，對應
  `npm run test:unit`。測試檔為 `src/**/*.test.ts`；純邏輯（`lib/utils`、`lib/stores`、`lib/api`）
  與元件渲染（`.svelte`）皆可測。目前為框架＋種子測試，覆蓋逐步補齊。
- **設定分離**：測試用獨立的 `vitest.config.ts`（不帶 `vite.config.ts` 的 build `outDir` /
  `emptyOutDir` / manualChunks，避免跑測試誤動 dist 產物）；`resolve.conditions=['browser']` 讓
  Svelte 元件能在 jsdom 掛載；`vitest-setup.ts` 載入 jest-dom matcher 並手動 `afterEach(cleanup)`。
- **記憶體**：2GiB host 下 vitest 以 `pool: 'forks'` + `singleFork` 限制併發（比照 Go
  `GOMAXPROCS=1`、Python `-p=1`）；`MEM` 預設 1024m，vitest+jsdom 較吃資源時可經環境變數上調。
- **版本相容**：Svelte 4 需 `svelte-check@^3`（v4 需 Svelte 5）與 `@testing-library/svelte@^4`
  （v5 的 `svelteTesting` vite plugin 需 Svelte 5，本專案不適用，故手動設定 browser condition
  與 cleanup）。

Backend image build 的記憶體約束：`backend/Dockerfile` 的 builder stage 固定
`GOFLAGS=-p=1`、`GOMAXPROCS=1`、`GOGC=off`、`GOMEMLIMIT=250MiB`。這台 host 只有 2GiB RAM
（實際可用約 700MB），沒有這些設定時 `redis/go-redis`、`modernc.org/libc` 的 compile 會被
OOM killer 砍掉（`signal: killed`）；實測光是序列化還不夠，`GOMEMLIMIT=500MiB` 仍失敗，
壓到 250MiB 才能在冷 cache 下編完（約 3 分鐘）。這些只影響編譯過程，不影響產出的執行檔。

### 2. 啟動 dev stack 做 smoke test

`docker-compose.dev.yml` 已對 dev stack 服務套用同樣的資源限制：
每個 container 預設最多 `0.5` CPU、`512m` 實體記憶體、`768m` memory+swap、`200` 個
process/thread。

```bash
scripts/smoke-dev.sh
```

腳本流程是 **先停 → 再 build → 再啟動**：build 之前會對 dev project 執行
`compose down --remove-orphans`（**不帶 `-v`，named volume 保留**）。原因是這台 host 只有
2GiB RAM，實測冷 cache build 的低點只剩 74 MiB available（Go compile 峰值 RSS 約 420 MiB），
而上一輪留著的 dev stack 約占 145 MiB（postgres 26＋redis 9＋backend 9＋python-server 99），
不先停就會在 build 階段被 OOM killer 砍掉。確定記憶體充裕時可用 `SKIP_DOWN=1` 略過這一步。

可用環境變數覆寫等待時間、log 行數或 health URL：

```bash
WAIT_SECONDS=120 LOG_TAIL=200 scripts/smoke-dev.sh
BACKEND_URL=http://localhost:18080/health PYTHON_URL=http://localhost:18001/health scripts/smoke-dev.sh
```

需要手動查看狀態或 log 時：

```bash
docker compose -f docker-compose.dev.yml ps
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

- 受影響 runtime 的測試腳本（`backend|python|frontend/scripts/test.sh`）。
- 若有 migration、API、跨服務整合、排程或 Python/Go 互動，跑 `scripts/smoke-dev.sh` 做 dev stack smoke test。
- 若有前端畫面變更，跑 frontend Docker build，並在 dev stack 或本地 dev server 驗證畫面。
- 若因環境、網路或外部 token 無法執行某項驗證，最後回報要明確寫出未執行項目與原因。

宣告完成或移除 issue/todo 項目前，逐項走過下方「結案確認清單（Definition of Done）」。

## 結案確認清單（Definition of Done）

「開發完成標準」是最低要求，這份清單是**宣告任務完成前（或把 issue/todo 項目移除前）逐項確認的
操作版**。每一條都對應本專案實際踩過的結案缺陷，別跳過。整份走完再說「完成」。

### A. 測試驗證

- [ ] 受影響 runtime 的 `scripts/test.sh` 全綠，且用 `-count=1`（或等效）跑過一次，不靠 cache 假綠。
- [ ] 新增／修改的邏輯**每個分支**都有斷言。曾發生新的 semantic action / position context 分支
      （例如 `DEFEND_BREAKDOWN`、`POSITION_SUPPORT_DEFENSE`、`POSITION_RESISTANCE_OVERHEAD`）只實作沒測到。
- [ ] 期望值來自**規格**、且測的是 **production 真的會產生的輸入**。不要手工捏造 production
      永不出現的分歧來「驗證」一個實際空轉的能力（`final_entry_gate_state` echo 的教訓）。見品質守則 §1。
- [ ] 若動到 migration／API／跨服務／排程／Python↔Go 互動，跑過 `scripts/smoke-dev.sh`。

### B. 文件收斂與狀態誠實

- [ ] 主題文件（`sr-zone-scoring.md` 等）與實作一致，**不得把「目標終局」寫成已完成**；只做一半就
      如實標「⚠️／規劃中」。曾把 annotation 層寫成「已達成單一真相源」。
- [ ] 完成的 `issue.md` / `todo.md` 項目已移除或搬到「已完成封存」；移除前把 durable 設計寫回主題文件，
      並**修掉其他文件指向該筆的交叉引用**（避免斷鏈）。見「文件收斂規則」。
- [ ] 狀態誠實：phased 工作在收尾前標「進行中」並保留剩餘 phase，**不要提前標「已完成」**。曾兩次把
      phased semantic pipeline 工作在只做到一半時標成完成。

### C. 前後端契約與一致性

- [ ] 新增分析欄位時 Python → Go(`internal/analysis/client.go`) → TS(`lib/api/*.ts`) 三端同步，
      新欄位用 `omitempty`／optional 保向後相容。見「SR Zone / 分析輸出欄位開發注意事項」。
- [ ] 新的 `decision_summary`／derived 欄位**已在前端接線或顯式延後**，不能只加型別不渲染。見品質守則 §3。
- [ ] 共用的 label／對照表抽到**單一模組**（例如 `derivedReasonLabel` 放 `srZones.ts`），不要兩個頁面各自
      維護一份而漂移。

### D. 不留 dead / echo / 雙真相源

- [ ] 沒有只是鏡像另一欄位的 echo 欄位（如 `final_entry_gate_state` = `entry_action_state`）。
- [ ] 沒有 legacy state 與 derived gate 並存的雙真相源；legacy 欄位要嘛由 gate 推導、要嘛退役並在文件
      標明哪個是權威、消費端不應讀 legacy。
- [ ] 重複邏輯已收斂到單一 helper（如 blocking-zone 偵測），避免兩份 copy 漂移。

### E. 產物與版控

- [ ] 前端有變更 → **重新 build dist 並 `git add backend/internal/ui/dist`**。`git commit -am` 不會帶
      未追蹤的新 chunk，漏了會讓 `index.html` 指向不存在的檔案、Go embed 前端 404。
- [ ] `git status` 只剩本次預期的改動：無 root-owned 檔案、無誤入的執行檔／`__pycache__`／快取殘渣。

### F. 回報

- [ ] 回報明確寫出「跑了什麼、結果、沒跑什麼與原因」；測試失敗或步驟略過要如實說，不要含糊帶過。

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
