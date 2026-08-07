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

### MySQL migration 驗證：`scripts/test-mysql-migrations.sh`

改到 `backend/internal/database/migrations/mysql/` 就要跑這支。dev / live 都用 postgres，
mysql 那份 migration 沒有其他執行路徑（背景見 [`issue.md`](./issue.md) I-054——
2026-08-07 首次實跑就抓到 5 個保留字語法錯誤）。

```bash
scripts/test-mysql-migrations.sh              # 起 MySQL → goose up → 驗 schema → down → 收掉
KEEP_UP=1 scripts/test-mysql-migrations.sh    # 跑完保留 MySQL（可從 127.0.0.1:13306 連進去看）
```

驗證邏輯在 `backend/internal/database/migrate_mysql_test.go`，用的是與 `cmd/server/main.go`
相同的 `database.RunMigrations` 進入點（migration 是 `//go:embed` 打包的，
從磁碟讀檔的 goose CLI 驗的是另一份東西）。該測試以 `MYSQL_MIGRATION_DSN` gate 住，
沒設就 skip，所以 `backend/scripts/test.sh ./...` 不受影響。

**兩階段設計與記憶體實測（2026-08-07）**：腳本刻意先在 MySQL 還沒起來時把測試編成執行檔，
編譯器退場後才起 MySQL，再用輕量 container 跑編好的 binary——峰值因此是
max(編譯, MySQL) 而不是 sum。調瘦後（`performance-schema=OFF`、buffer pool 64M）
MySQL 實測佔 **182MiB**（預設值會是 400MB 以上），過程中 host available 低點約 689MB。
腳本開頭會檢查 available ≥ 600MB，不足直接中止而不是硬跑。

`down` 只回滾到版本 17，不是 0——原因見 [`issue.md`](./issue.md) **I-057**（017／018 的
Down 直接 DROP 而不還原舊結構，是三個 engine 共有的結構性問題）。

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

# 開發迭代：只跑指定測試檔（略過 svelte-check 與 build）。驗收仍要跑不帶此變數的完整三步。
VITEST_ARGS="src/routes/SRZones.test.ts" frontend/scripts/test.sh
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
| `MEM`（記憶體上限） | `700m` | `700m` | `440m`（每步一個 container） |
| `MEMSWAP` | 同 `MEM` | 同 `MEM` | 同 `MEM` |
| `CPUS` | `1` | `1` | `1` |
| `--pids-limit` | 200 | 200 | 200 |
| image 覆寫 | `GO_IMAGE` | `PY_IMAGE` | `NODE_IMAGE` |

### `MEM` 是上限，不是預留——不可高於 host 可用量

三支腳本都在 `docker run` 前呼叫 `scripts/lib/mem-guard.sh` 的 `mem_guard_clamp`，把 `MEM`
壓進 host 當下真的供得起的範圍：上限 = `/proc/meminfo` 的 **`MemAvailable`**（不是 `MemTotal`）
減 `MEM_RESERVE_MB`（預設 150）。超過就印警告並自動下修。

**為什麼要有這道護欄**：`--memory` 是 cgroup 的上限，不是向 host 預留記憶體。設得比
`MemAvailable` 高時，container 根本撞不到自己的限制，會先耗盡 host 實體記憶體 + swap，
於是由 **host 層級的 OOM killer** 出手，砍掉 badness 分數最高的行程——在這台機器上就是
**呼叫測試的工作階段本身**（claude CLI，RSS 隨對話成長到 400~500MB）。2026-08-03 與
2026-08-05 各發生一次；2026-08-05 那次的 container 還活過呼叫端並自己跑完（測試全綠，log
留在 scratchpad），死的是呼叫測試的人——**被 kill 後先去撈 log，不要反射性重跑**。

所以遇到記憶體不足時，**要降的是實際用量，不是把 `MEM` 調大**。調大只是把「container 撞
自己的 cgroup 上限」（可回收、錯誤訊息讀得到、工作階段活著）換成「host 砍掉呼叫端」
（整個工作階段連同未回報的結果一起消失）。

### container 上限的**總和**也要顧——本機同時只留一組 stack

同一個失效模式不只來自測試腳本的 `--memory`，也來自**常駐 compose stack 的 `mem_limit`
總和**。每個 stack 都設 `0.5` CPU / `512m` / `768m`，看似安全，但那是**每個 container 各自的
上限**，不是總量保證：10 個 container 加起來就是 ~5 GB 的授權額度，而 host 只有 2 GiB。

2026-08-05 16:22 實際發生過（見 [issue.md](./issue.md) I-053）：live project `stock_trading`
被拉起來後，全機共 10 個 container ＋ claude ~400 MB ＋ codex ~137 MB ＋ dockerd ~314 MB，
**沒有任何 container 撞到自己的 512m 上限**（全部 `OOMKilled=false`），是 host 先耗盡，
於是 claude 被砍，接著 `docker-proxy` × 8 與 `dockerd` 也被砍，所有 container 一起停掉。

規則：

- **本機同時只允許一組 stack 常駐**。開工前先 `docker ps --format '{{.Names}}'` 看一眼，
  多餘的先 `compose down`（不帶 `-v`）。
- **不要在本機把 live/deploy project 拉起來**——驗收一律用 `docker-compose.dev.yml` 的
  dev project（CLAUDE.md 規定）。
- 開跑前 `free -m` 的 `available` 低於 ~800 MB 就先清場再說，不要硬上。

### frontend 三步的記憶體實測（2026-08-06）

`frontend/scripts/test.sh` 的三步裡 **svelte-check 是瓶頸，而且它的用量幾乎壓不下去**。
2026-08-06 逐項量測的結果：

| 嘗試 | 結果 |
|---|---|
| `--max-old-space-size` 180 / 200 / 215 | 全部 OOM |
| `--max-old-space-size` 230 | 通過 → **下限抓 225MB heap** |
| `tsconfig.json` 加 `skipLibCheck: true` | tsc program 由 130.5MB 降到 100.8MB（−23%）、check 時間 9.60s→2.15s（快 4.5×）。**但 svelte-check 的下限幾乎沒降**——svelte 的 language service 才是大宗，不是那 881 個 `.d.ts` |
| `--diagnostic-sources js,svelte`（去掉 css） | heap 200MB 仍 OOM，無效 |
| `--max-semi-space-size=2` | 有沒有它都一樣，無效 |

換算成前置條件：**heap 225 → `MEM` ≥ 330m → host `MemAvailable` ≥ 約 480MB**
（heap = MEM − 100，MEM 再被 mem-guard 減去 150MB 的 reserve）。這台平常在 450~510MB
之間浮動，**正好卡在門檻上**，所以同一份程式碼可能這次過、下次 OOM——不是 flaky，是記憶體。

腳本已在 svelte-check 前加一道警告：clamp 後的 heap 低於 `SVELTE_CHECK_MIN_HEAP_MB`（225）
時明講「很可能 OOM，要解的是釋放 host 記憶體」。**只警告不中止**，因為下限是區間值、
邊界上仍可能過。只想跑單元測試時用 `VITEST_ARGS=...` 略過這一步。

`skipLibCheck` 雖然不解 OOM，仍值得留著：省 30MB 的 margin 與 4.5 倍的 check 速度。
代價是不再檢查函式庫 `.d.ts` 內部的型別錯誤（例如兩個套件的 global 型別互相衝突）；
自己的程式碼對照函式庫型別的檢查完全不受影響，這也是 Vite / Svelte 官方 template 的預設。

### 事後判讀 OOM：這台的 `dmesg -T` 時間不可信

調查被 kill 的原因時，`dmesg -T` 的**絕對時間會錯**（此沙箱 kernel 單調時鐘與 wallclock 有
數十小時偏移，2026-08-05 實測 ~44.8 小時，會把當天的事件標成兩天前）。正確做法：

- 只用 `dmesg` 的**相對間隔**，再拿 `docker inspect -f '{{.State.StartedAt}} {{.State.FinishedAt}}'`
  （docker 用自己的 wallclock，可信）對齊到真實時間。
- 決定性驗證：`dmesg` 最後一行的 veth 名稱若等於 `ip -o link` 目前唯一存在的 veth，
  該行就是最近一次 container 啟動的時點。
- kernel ring buffer 只留約 60 行 / 1.7 小時，查不到不等於沒發生。

護欄開關（環境變數）：

| 變數 | 預設 | 作用 |
|------|------|------|
| `MEM_RESERVE_MB` | `150` | 保留給呼叫端**執行期間再成長**的量。`MemAvailable` 已扣掉各行程當下的 RSS，不需在此重複涵蓋；設太大會讓上限低到無法執行 |
| `MEM_MIN_MB` | `256` | 下修後低於此值直接中止，並提示先關掉常駐 container |
| `MEM_STRICT` | `0` | 設 `1` 時不下修，直接中止（想明確發現設定錯誤時用） |
| `MEM_FORCE` | `0` | 設 `1` 時完全略過護欄 |

`MEMSWAP`（`--memory-swap`）預設等於 `MEM`，即**關掉 container 的 swap**。這台 host 的 512MB
swap 常態 100% 用滿，放任 container 換頁只會拖垮整台機器，不如讓它乾脆撞 cgroup 上限。

Go 另外固定 `GOMAXPROCS=1` + `GOFLAGS=-p=1`：本機只有 2GiB RAM，平行編譯會 OOM，
必須序列編譯；記憶體上限也因此不能沿用其他 runtime 的 512m。再加上 `GOGC=off` +
`GOMEMLIMIT`（腳本的 `GO_MEMLIMIT`，預設 `250MiB`，可經環境變數上調）：序列化只限制
**併發數**，不限制單一 go 子行程的 heap，container 的 `--memory` 也擋不住 host 層級的
OOM killer。沒有這兩項時，`modernc.org/sqlite/lib`（C 轉譯的巨大 generated package）的
vet／compile 會出現 `vet: signal: killed`，`set -e` 讓整條 vet → test → build 直接中止。
這些只影響編譯過程，不影響測試結果與產出的執行檔。

Frontend 注意事項：`vite.config.ts` 的 `outDir` 是 `backend/internal/ui/dist`
（Go embed 使用、且有進版控），所以腳本掛載的是 **repo root** 而非 `frontend/`。
跑完 `git status` 出現 dist 差異屬正常，要不要保留該次產物由當次工作決定。

Frontend 顏色語意（`tailwind.config.js`）：`rise: #e74c3c`（**紅**）、`fall: #2ecc71`（**綠**），
名稱來自台股「漲紅跌綠」。**關鍵陷阱：`fall` 是綠色**，不要因為「fall 聽起來像壞事」就拿去標
錯誤或危險操作，那會把警示顯示成安全色。用法分三類：

| 情境 | 用什麼 | 例子 |
|------|--------|------|
| 行情語意 | `text-rise` / `text-fall` | 漲跌、損益、買賣超、停損價、最大回撤 |
| 錯誤／失敗訊息文字 | `text-rise`（紅） | job error、觸發失敗、載入失敗 |
| UI 狀態徽章與動作按鈕 | tailwind 色票 `text-red-400` / `text-green-400` 等 | 刪除／取消／停用按鈕、status chip |

第二類沿用 `text-rise` 是既有慣例（`SRZones.svelte`、`Scheduler.svelte` 的載入錯誤本來就這樣）；
第三類用色票是因為按鈕本來就與行情無關，且同組按鈕的另一分支已經是
`border-green-600/40 text-green-400`，用色票才對稱。

這個坑實際發生過兩次（2026-08-05 修正）：`Scheduler.svelte` 整檔把 job 錯誤與失敗數標成綠色
（同檔的載入錯誤卻是紅色，自相矛盾）；`Users.svelte` / `Backtest.svelte` / `Analysis.svelte`
的停用／取消／刪除按鈕也是綠色，其中 `Users.svelte` 的切換按鈕**兩個狀態都是綠色**，使用者
完全分不出哪邊是危險操作。已在 `Scheduler.test.ts` / `SRZones.test.ts` / `Users.test.ts` /
`Backtest.test.ts` / `Analysis.test.ts` 加上 class 斷言鎖住——**只斷言文字內容的測試抓不到這種錯**。

Frontend 測試框架（三層）：

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
  `GOMAXPROCS=1`、Python `-p=1`）。腳本另外把三步**拆成三個獨立 container** 依序執行
  （`npm run check` → `npm run test:unit` → `npm run build`）：每步跑完就退出、記憶體立刻
  歸還 host，峰值變成三者的 **max 而非 sum**，哪一步爆掉也一眼可辨。再加
  `NODE_OPTIONS=--max-old-space-size`（腳本的 `NODE_HEAP_MB`，預設 320）——node 的預設
  old-space 由可用記憶體推導，不明確指定就會一路漲到接近 cgroup 上限才認真 GC，這是把實際
  用量壓下來的主要槓桿。2026-08-05 實測三步峰值（`memory.max_usage_in_bytes`，含可回收的
  page cache）：svelte-check 376MB、vitest 359MB、vite build 370MB；`MEM` 預設 440m，被護欄
  下修到 365m 時三步仍全過（緊縮時 kernel 直接回收 page cache，不會 OOM）。
  **下限**：`svelte-check` 的 node heap 低於約 200MB 會直接 `JavaScript heap out of memory`
  （實測 198m 失敗、231m 通過）。host 吃緊到 `MEM` 被下修至 300m 以下時就會踩到，這時不是
  改 code，是等記憶體回來或調 `MEM_RESERVE_MB`。失敗發生在 container 內，呼叫端不受影響。
  **不要因為想給測試更多空間而調高 `MEM`**，理由見上面的「`MEM` 是上限，不是預留」。
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
- 若改到 `migrations/mysql/`，另外跑 `scripts/test-mysql-migrations.sh`。
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
- [ ] 若動到 `migrations/mysql/`，另外跑過 `scripts/test-mysql-migrations.sh`（dev/live 是
      postgres，mysql 那份沒有其他執行路徑）。

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
- **沒被消費的型別還會默默寫錯**（2026-08-06，T-028 前端接入）：
  `SRDailyConfirmationSummary.by_state` / `by_primary_role` 一度被宣告成
  `SRDecisionOutcomeGroup`，但 Python `_daily_confirmation_groups` 實際回傳的形狀與它
  **除了 `rows` 之外零重疊**。因為從沒有任何地方消費這兩個欄位，`svelte-check` 與 build 都
  不會比對它跟真實資料——型別看起來「有寫」，其實是錯的。這比單純漏渲染更難發現：漏渲染至少
  肉眼看得出畫面少東西，型別錯了則要等到有人真的去用才會炸。
  **所以「型別已宣告」不能當成進度**，只有被渲染且有測試斷言過的欄位才算接線完成。

  同理，新增分層／統計欄位時不要在前端自行推導比率。該批分層只提供原始 counts，Python 的
  `_outcome_rate` 帶 `primary_role` 過濾語意，前端相除得到的數字會跟後端定義悄悄分岔，
  且不會有任何測試發現。要比率就在 Python 算好送過來。
- **測試 fixture 必須是後端真的會產生的形狀，斷言必須到值**（2026-08-06，issue.md I-055）：
  `zone_outcomes` 的分層比率在前端永遠顯示 `—` 長達數週，因為 Python 回的是
  `hold_rate`/`break_rate`、前端讀的是 `support_hold_rate` 等三個不存在的 key。
  三層測試全綠是因為——**前端測試的 fixture 是憑印象手寫的**，用了後端從不產生的 key，
  於是「測試通過的是一份不存在的資料形狀」；而 Python 測試只斷言分層非空與 rows 加總，
  剛好完全避開出錯的欄位。`—` 又看起來就像「這組沒資料」，肉眼也發現不了。

  兩條具體要求：

  1. **fixture 從真實輸出取樣**，不要憑記憶手寫。跑一次真的產出、複製其中一段當 fixture。
  2. **斷言要到值**，不要只斷言區塊／表格存在。`getByText(/Zone 層指標/)` 這種斷言在三個欄位
     全是 `—` 時照樣通過；鎖到該列的 `<td>` 斷言「出現 62.0%」才擋得住。
     同理，null 的欄位要斷言它顯示 `—` **且不出現 `0.0%`**。
  3. 跨語言的欄位契約，**最終要靠一次真實資料的實跑驗證**——單元測試兩邊都用合成資料時，
     兩邊可以一致地錯。I-055 就是靠 T-039 的 Pass 0 實跑才浮出來。

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
- **FastAPI 服務的「進入點」＝ lifespan，不是 module 頂層**（2026-08-06，T-037 C）：
  `http_server.py` 原本在 module 頂層直接呼叫 `check_connection()`，等於「import 這個模組 ==
  必須連得到 DB」，讓 `/sr-scoring/evaluate` 完全無法用 FastAPI TestClient 測（該端點因此長期
  0 測試）。現改成 `@asynccontextmanager` 的 `lifespan`，掛進 `FastAPI(lifespan=...)`：

  ```python
  @asynccontextmanager
  async def lifespan(app: FastAPI):
      check_connection()
      yield

  app = FastAPI(..., lifespan=lifespan)
  ```

  兩條啟動路徑（compose 的 `python http_server.py` → 檔尾 `uvicorn.run(app)`、`start_server.sh`
  的 `uvicorn http_server:app`）都會經過 lifespan，**fail-fast 行為不變**：已實測連不到 DB 時
  uvicorn 記 `Application startup failed. Exiting.` 並以 **exit 3** 退出，container 照樣依
  `restart: unless-stopped` 重啟。
- **測試側的對應寫法**：starlette 的 `TestClient` **只有被當成 context manager 使用時才會跑
  lifespan**。所以端點測試一律用 `TestClient(app)`（不加 `with`）＝ 完全不需要 DB；只有要驗證
  啟動行為的測試才寫 `with TestClient(app):`。這點寫在 `python/tests/conftest.py` 的 `client`
  fixture 註解裡——若有人順手改成 `with`，整批端點測試會突然需要 DB。

## 文件收斂規則

發現與實作不一致時：

- bug、矛盾結果、誤導行為、文件與實作不一致、已知限制：記到 `docs/issue.md`。
- 未來優化、功能擴充、重構、待規劃工作：記到 `docs/todo.md`。
- 已完成的 issue/todo 要移除；移除前，把值得長期保存的行為或設計寫回對應主題文件。
