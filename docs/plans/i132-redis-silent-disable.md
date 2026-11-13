# I-132：backend 沒帶到 Redis 密碼就靜默停用 Redis——部署範本版控與啟動檢查（計畫書 v3）

> 狀態：v3 草稿，待使用者確認（2026-11-13）。使用者已定案：D2、D7、D8；D1、D3～D6、D9～D11 是 Claude 的建議，**還沒經過使用者確認**。⛔ 確認前不改程式、不改範本、不對 live 下指令。
> ▶ 2026-11-13 恢復（I-129、I-131、I-124～I-126 處理完之後）。C.3 的 Q1～Q4 都已由使用者決定（Q1：live 的 JWT 已換成 64 字元）。下一步：附錄 C 的契約表 review 通過後開始寫 E 的程式。
> 對應：`docs/issue.md` I-132。v2 review 的原文與逐項處理見文末「Review feedback」與「十一、修訂紀錄」。

## 一、目標與不做的範圍

**目標**：

* **A（部署範本的版控）**：把 live 已經驗證過的做法同步回 repo 的範本。
  * A1（v3 同日改，D7）：`REDIS_PASSWORD` ⛔ 不再經過 shell 和 `${}` 展開，改由 backend 的 `env_file:` 直接讀 Redis stack 的 `.env`，`environment:` 裡⛔ 不寫 `REDIS_PASSWORD`。這樣手動執行 `docker compose logs`／`ps` 也不必先帶入密碼（F3 解決）。原本的 `:?` 防呆由 E 的程式檢查取代。
  * A2：新增 `.env.example`。backend 的應用設定改由 `env_file:` 注入，路徑用絕對路徑（4.1、D10）；`environment:` 裡⛔ 不再寫同名的鍵（D2）。
  * A3：`deploy.sh` 範本拿掉所有 `export` 的設定，改成「任何一步失敗就停下、不進 `down`／`up`」的流程，加上部署前檢查（`.env` 的位置與權限、最終設定裡的 JWT 不能是空值或公開範例值）和部署後的 readiness 確認（4.3、D5、D6）。
* **E（設定契約檢查，v3 同日新增，D8）**：backend 載入設定之後、連 DB 之前，依一份完整的設定契約檢查必要的設定是否齊全、是否還是公開的範例值，不齊就以非 0 結束，log 只寫違反了哪個鍵、哪一條規則，⛔ 不寫值（4.6）。
* **C（程式修正）**：backend 啟動時，如果有設定 `REDIS_ADDR` 但連不上 Redis，用有時間上限的啟動探測重試幾次（4.4），仍然失敗就在 HTTP 和排程器啟動之前以非 0 結束，⛔ 不再靜默停用。沒有設定 `REDIS_ADDR` 時照舊允許停用。

**不做**：

* **B（另外加防呆）**：原本是靠 A1 的 `:?` 擋住「沒帶密碼就直接 `docker compose up -d`」，所以不另外處理（使用者 2026-11-13 決定）。v3 同日改用 `env_file` 之後，這層保護改由 E 的程式檢查負責。
* backend 執行期間 Redis 中途斷線的行為不改：runtime 用的 client 設定一字不改，照舊由 client 自動重連。C 只新增一個啟動用的探測，用完就關閉。
* dev compose 的 Redis 密碼機制不改（預設 `dev-redis-only`）。
* repo 範本和 live 的其他結構差異：DB 憑證的 `env_file`、資源限制、log 的絕對路徑、external networks（D1）。這些另外開 todo。
* python-worker／python-server 的設定契約：它們只讀 DB 連線和模型路徑，DB 連不上時 lifespan 本來就會 fail-fast，所以本案只做 backend。之後要擴充的話另開 todo。
* live 的修改由使用者執行，Claude 只給指令和預期輸出。

## 二、背景與事實（2026-11-13 查證）

### 2.1 兩次事故

| 日期 | 主機 | 經過 | 停用時間 |
|---|---|---|---|
| 2026-11-02 | 舊機 | I-123～I-126 上 live 時，backend 連續 3 次啟動都是 `NOAUTH` | 11-02 17:39～11-03 10:58 |
| 2026-11-12 | 新機 | 16:28:37 用 `docker compose up` 重建 backend，⛔ 沒有經過 `deploy.sh` | 11-12 16:28～11-13 11:21 |

* 共同原因：`REDIS_PASSWORD` 只放在 Redis stack 的 `.env`，只有 `deploy.sh` 的白名單區塊會讀出來並 `export`。不經過 `deploy.sh` 的重啟，compose 的 `${REDIS_PASSWORD:-}` 就會帶入空字串。
* 為什麼沒人發現：backend 只在啟動時連一次 Redis，失敗就記 `redis connect failed, cache disabled`，然後整個停用。API、排程、`job_runs` 都照常運作，只有指標快取和訊號佇列靜默失效。
* 11-13 的修正與核對（原記在 I-131 計畫書附錄 B；I-131 收斂時搬到這裡，原文見 `git show aec6bbe:docs/plans/old-vps-data-merge.md`）：
  * 新 live 的 backend 在 11-12 11:57:46、14:23:34、14:29:51、16:28:37 共啟動四次，前三次都是 `redis connected`，**16:28:37 那次是 `redis connect failed, cache disabled`，錯誤類別 `NOAUTH`**（使用者用只印訊息名稱和錯誤類別的指令確認）。
  * 原因：16:28 那次沒有經過 `deploy.sh`，專案目錄的 `.env` 不含 `REDIS_PASSWORD`，帶進去的是空字串。
  * 11-13 11:21:32 修正：照 `deploy.sh` 同一套白名單從 Redis 的 `.env` 讀出密碼，只重建 backend。重建後核對：image 不變、其餘環境變數的雜湊不變、密碼雜湊跟 Redis 一致、`redis connected`；Redis 上看得到 backend 的連線，`indicator:*:latest` 每一輪都有更新；重建沒有中斷任何工作（11:20 那一輪在重建前就結束了）。

### 2.2 live 的現況（2026-11-13，已依 D2 修正）

| 檔案 | 現況 |
|---|---|
| `/opt/stacks/scripts/stock_trading/compose.yml`（SHA-256 `f75a32bd…`；依序是 `affec70c…` → `20825bfc…`（D2）→ `f75a32bd…`（D7）） | `environment:` ⛔ 沒有 `REDIS_PASSWORD`，compose 檔裡也沒有必填的 `${}`。backend 的 `env_file` 有三項：DB 憑證（`format: raw`）、`/opt/stacks/scripts/stock_trading/.env`、`/opt/stacks/scripts/redis/.env`（後兩項 `required: true`，固定絕對路徑）。`environment:` 已刪掉和 `.env` 同名的 19 個鍵；其他設定（例如 `YAHOO_RATE_LIMIT`、`CANDLE_GAP_DETECTION_*` 的細項）還留在 `environment:`，帶 compose 的預設值。三個服務都有資源限制；log 掛絕對路徑；networks 是 external |
| `/opt/stacks/scripts/stock_trading/.env`（權限 `600`） | 19 個鍵，包含 `AUTH_JWT_SECRET`、`FINMIND_API_KEY`、`FUGLE_API_KEY` 等機密，以及開關和 cron。⛔ 沒有 `REDIS_PASSWORD` |
| `/opt/stacks/scripts/stock_trading/deploy.sh`（SHA-256 `07ebcc55…`；原本是 `5d569e06…`） | Redis 白名單區塊 →（新增）`.env` 可讀且權限 600 的檢查 → `cd $PROJECT_DIR` → `git pull origin init` → `cd $SCRIPT_DIR` → `down` → `up --build -d`。compose 用 `-f ./compose.yml --project-directory "$PROJECT_DIR"`，`PROJECT_DIR` 是 build 目錄。⚠️ 沒有 `set -e`，`cd`／`git pull`／`down` 失敗也會繼續往下（D6） |

repo 的範本（2026-11-13）：

| 檔案 | 現況 |
|---|---|
| `docker-compose.yml`（SHA-256 `60e2689b…`） | `REDIS_PASSWORD: ${REDIS_PASSWORD:-}`；DSN 寫死 `trading:trading`；沒有 `env_file`、沒有資源限制；log 用相對路徑 |
| `deploy.sh`（SHA-256 `225a6a8c…`） | 一長串 `export`（`AUTH_JWT_SECRET="change-me"` 等）、Redis 白名單區塊、`down`、`up --build -d`。⛔ 沒有 `git pull`，也沒有失敗即停。compose 用 `-f "$PROJECT_DIR/docker-compose.yml" --project-directory "$PROJECT_DIR"` |

所以 live 和 repo 範本的部署流程**不一樣**：live 多了 `git pull`，而且 compose 檔是 scripts 目錄裡的 `compose.yml`。v3 的範本採用 live 的流程（4.3）。

### 2.3 查證時發現的其他問題

* **F1 `.env` 讀錯目錄（✅ live 已結案）**：compose 預設從 project directory 找 `.env`，`deploy.sh` 的 project directory 是 build 目錄，但 `.env` 放在 scripts 目錄。
  * E1：build 目錄⛔ 沒有 `.env`。
  * A/B 實測：照 `deploy.sh` 的寫法跑 `config`，五個開關都展開成 `"false"`、`AUTH_JWT_SECRET` 和 `FINMIND_API_KEY` 是空字串、`is not set` 警告 3 則；加上 `--env-file` 就變成 `"true"`、有值、0 則。
  * 實際後果比空值更嚴重：backend 用 viper 的 `AutomaticEnv`，空字串會被當成沒設定，退回 `config.yaml` 的預設值。`AUTH_JWT_SECRET` 會變成 repo 裡公開的 `change-me-in-production`，所有人都能偽造 token；`FINMIND_API_KEY` 會變成 `YOUR_FINMIND_API_KEY`。這些⛔ 都不會報錯（D5）。
  * 處置（D2，使用者決定）：live 的 backend 改用 `env_file` 讀 `.env`，並刪掉 `environment:` 裡同名的鍵。修改前後的 `config` 比對一致，第一次部署前後 container 的 `Config.Env` 也一致。經過見第八節第 1 步。
* **F2 `.env` 權限**：原本 `664`，✅ 已改成 `600`。
* **F3 `:?` 的副作用**：compose 每個子指令（`ps`、`logs`、`down`、`exec`）都會解析整份檔案，所以都要先帶入 `REDIS_PASSWORD`，否則直接報錯（2026-11-13 使用者實際遇到：`docker compose logs` 失敗）。**v3 同日的處置（D7）**：`REDIS_PASSWORD` 改由 `env_file` 讀 Redis stack 的 `.env`，compose 檔裡就沒有必填的 `${}` 了，手動指令不必再帶任何東西。
* **F4 兩種 project directory**：手動操作用 scripts 目錄，`deploy.sh` 用 build 目錄。兩者的 basename 都是 `stock_trading`，所以操作的是同一組 container，但相對路徑會解析到不同地方（例如 `build.context: .`）。live 的 `.env` 已經改用絕對路徑，不再受影響；build context 的部分維持「⛔ 不在 scripts 目錄手動 `up --build`」。

### 2.3.1 `env_file` 的相對路徑（2026-11-13 在 dev 實測，Compose v5.7.0）

`env_file` 寫相對路徑（例如 `./.env`）時，是相對於 **project directory** 解析，⛔ 不是相對於 compose 檔所在的目錄。實測：compose 檔放在 scripts 目錄、`--project-directory` 指向 build 目錄，讀到的是 build 目錄的 `.env`。所以 `deploy.sh` 那種「compose 檔在 scripts 目錄、project directory 是 build 目錄」的用法，`env_file` 一定要用**絕對路徑**（D10）。

### 2.4 backend 的啟動順序（2026-11-13 讀程式碼確認）

`main.go` 的順序是：載入設定 →（E 的設定契約檢查，新增）→ 連 DB → 自動套用 migration → **Redis** → 建立 repo 和 service → `AbortRunning` 回收孤兒紀錄 → `go sched.Start()` → HTTP 開始服務。

* 所以 C 在 Redis 這一步結束程序時，`AbortRunning`、排程器、HTTP 都還沒啟動，⛔ 不會寫 `job_runs`，也⛔ 不會對外提供 API。
* migration 已經跑過了。migration 本身是冪等的，container 反覆重啟時每次都會檢查一次，這沒有問題。

### 2.5 go-redis 的預設行為（v9.5.4）

* repo 用的是 `github.com/redis/go-redis/v9 v9.5.4`。`NewRedis` 只設定 `Addr`、`Password`、`DB`，其他全用預設值。
* 依 go-redis v9 的 `Options` 說明，預設值是：`MaxRetries` 重試 3 次（設成 `-1` 才是關閉）、`DialTimeout` 5 秒、`ReadTimeout` 3 秒、`WriteTimeout` 跟 `ReadTimeout` 相同、`PoolTimeout` 是 `ReadTimeout` + 1 秒、`ContextTimeoutEnabled` 是 false，也就是 socket 讀寫的逾時不看 context 的 deadline。
* 所以現在的一次 `Ping`，遇到連線逾時或拒絕連線這類可重試的錯誤，實際可能要花十幾秒以上。只在外面包 5 次迴圈，或只傳一個 3 秒的 context，都**證明不了**「單次探測最多 3 秒」（v2 review P2）。
* ⚠️ 上面的預設值是依照套件的公開說明整理的。實作時要讀 module cache 裡 v9.5.4 的原始碼再確認一次，並且用 dev 的實測時間（第七節）佐證。

## 三、受影響的檔案與資料流

| 檔案 | 變更 |
|---|---|
| `docker-compose.yml`（範本） | A1：`environment:` 刪掉 `REDIS_PASSWORD`，改由 `env_file` 讀 Redis stack 的 `.env`。A2：backend 的 `env_file` 加上應用設定的 `.env`（都用絕對路徑，D10），`environment:` 刪掉所有移到 `.env.example` 的鍵 |
| `.env.example`（新增） | A2：`deploy.sh` 範本原本 `export` 的每一個應用設定鍵（`REDIS_PASSWORD` 除外），連同說明。機密留空，開關照 repo 的保守預設（`false`） |
| `deploy.sh`（範本） | A3：拿掉 `export`；加上失敗即停、preflight、`git pull --ff-only`、readiness 確認（4.3） |
| `scripts/test-deploy-sh.sh`（新增） | 用假的 `git`／`docker` 驗證 `deploy.sh` 的呼叫順序和失敗即停；用真的 `docker compose config` 驗證設定解析（第七節） |
| `backend/internal/store/` 的 Redis 啟動探測（新增函式與單元測試） | C：有時間上限的探測和重試，錯誤分成封閉值域的原因類別 |
| `backend/internal/config/` 的設定契約（新增函式與單元測試） | E：契約表與檢查函式，回傳違反清單（鍵名＋規則代號，⛔ 不含值） |
| `backend/cmd/server/main.go` | E：載入設定之後、連 DB 之前做契約檢查，不通過就 `os.Exit(1)`；另外加一個只檢查設定、不連 DB 的模式（D11）。C：Redis 那一步改呼叫探測；全部失敗就記 Error 並 `os.Exit(1)` |
| `docker-compose.dev.yml` | E：backend 加上環境類別（D9 定案後的鍵，例如 `APP_ENV: dev`），讓 dev 的範例值可以通過檢查 |
| `docs/development-workflow.md` | 「live 的 Redis」「只改 live 的環境變數」「新增排程開關要改的地方」改成 `env_file`／`.env` 的現況；加上 F3 的標準載入指令和 Compose 最低版本 |
| `docs/architecture.md` | 「生產 / Docker」補上設定的來源和 Redis 的啟動語意 |
| `docs/todo.md` T-076 | 文中「`deploy.sh` 範本加 export」改成「`.env.example` 加鍵」 |
| `docs/todo.md`（新增一筆，D1） | repo 範本跟 live 結構差異的同步：DB 憑證、資源限制、log 路徑、networks |

**資料流的變化**：

* 應用設定：從「`deploy.sh` 用 `export` 帶進 shell → compose 用 `${}` 展開進 `environment:`」，改成「`.env`（compose 檔裡寫絕對路徑）→ compose 的 `env_file:` 直接注入 backend」。`.env` ⛔ 不再當成 `${}` 展開的來源。
* `REDIS_PASSWORD`（v3 同日改，D7）：從「`deploy.sh` 用白名單讀進 shell → compose 用 `:?` 展開」改成「compose 的 `env_file` 直接讀 Redis stack 的 `.env`」。值仍然只有 Redis 那一份，Redis stack 自己也讀同一份檔、用同一套 dotenv 規則解析。`deploy.sh` 的白名單保留下來當 preflight 檢查（那份檔只能有一行 `REDIS_PASSWORD`），⛔ 不再需要 `export`。
* 設定的檢查：從「compose 的 `:?`」改成「backend 啟動時的設定契約（E）」，不管用什麼方式啟動都會檢查。
* Redis 的啟動語意：「有設定位址也可能停用」改成「有設定位址，Redis 就是必要的」（4.4）。

## 四、設計

### 4.1 A1／A2：compose 範本

```yaml
services:
  backend:
    environment:
      # 只留固定的服務連線設定；⛔ 不寫 .env.example 裡有的鍵（environment 的優先順序比 env_file 高，會蓋掉）
      DATABASE_DRIVER: postgres
      DATABASE_DSN: …            # 照舊，DB 憑證的範本化屬於 D1 的後續
      REDIS_ADDR: redis:6379
      SERVER_PORT: "8080"
      PYTHON_SERVICE_URL: http://python-server:8001
      GIN_MODE: ${GIN_MODE:-}
      LOG_DIR: …
      LOG_RETENTION_DAYS: ${LOG_RETENTION_DAYS:-14}
    env_file:
      # 一定要用絕對路徑：相對路徑是相對於 project directory（build 目錄）解析（2.3.1）
      - path: /path/to/scripts/stock_trading/.env      # 應用設定（.env.example 複製過去）
        required: true
      - path: /path/to/redis-stack/.env                # 只有一行 REDIS_PASSWORD
        required: true
```

* 兩個 `env_file` 都用**絕對路徑**（D10，v3 同日改）：範本裡寫 `/path/to/…` 佔位，複製到 scripts 目錄時改成實際路徑，跟 `deploy.sh` 範本的 `PROJECT_DIR="/path/to/trading"` 同一套慣例。忘了改的話，`required: true` 會讓 compose 直接報錯。v3 原本的 `${DEPLOY_ENV_FILE:?…}` 不採用：它本身就是必填的 `${}`，手動指令又得先帶入變數，F3 會回來。
* `environment:` 裡⛔ 不寫 `REDIS_PASSWORD`：它的優先順序比 `env_file` 高，寫了就會蓋掉（就算展開成空字串也一樣）。
* `environment:` 保留哪些鍵要列清單，而且跟 `.env.example` 的鍵⛔ 不能重複。這由測試擋（第七節）。
* ⛔ 不加 `format: raw`：用 Compose 預設的 dotenv 解析，才會跟 live 現在的行為一樣（live 已驗證）。
* 範本檔名維持 `docker-compose.yml`。部署時複製到 scripts 目錄，命名為 `compose.yml`，跟 live 相同；`deploy.sh` 用變數 `STACK_COMPOSE_FILE` 指定，⛔ 不假設 repo 裡有 `compose.yml`。
* live 的 `compose.yml` 就是這個範本的實例：兩個 `env_file` 都是固定的絕對路徑（2026-11-13 改，第八節第 1、1b 步）。

### 4.2 A2：`.env.example`

* 開頭的說明：
  * 複製成 `.env`，放在 scripts 目錄（跟 `compose.yml`、`deploy.sh` 同一個目錄），`chmod 600`。
  * 一行一個 `KEY=value`，⛔ 不寫 `export`。⛔ 不要用 shell `source` 這個檔：compose 的解析規則跟 shell 不一樣。
  * 格式依 Compose 的 dotenv 規則：沒有引號的值，` #` 之後會被當成註解；`$` 會被展開。值裡有 `$`、`#`、引號或前後空白時，用**單引號**包起來（單引號內照字面讀）。這幾種寫法都要有測試（第七節）。
  * ⛔ 不放 `REDIS_PASSWORD`：它只存在 Redis stack 的 `.env`。
* 鍵的範圍：`deploy.sh` 範本現在 `export` 的每一個應用設定鍵，`REDIS_PASSWORD` 除外，⛔ 不能漏鍵（測試逐鍵比對）。
* 預設值：機密留空；開關照 repo 的慣例全部是 `false`。⛔ 不把 live 已經打開的開關照搬成範本的預設。數值和 cron 用 compose 現在的預設值。
* 每個鍵的說明，照搬 `deploy.sh` 範本現有的註解，例如公司行動沒有開關、終止上市對帳的依賴、重跑的回滾方式。
* ⚠️ 跟 live 的差異：live 的 `.env` 只有 19 個鍵，其他鍵還留在 live 的 `environment:`。範本則是把全部應用設定都放進 `.env.example`。兩邊的鍵都不重複，所以沒有衝突，但這個差異要寫進操作文件。

### 4.3 A3：`deploy.sh` 範本

流程（每一步失敗都回傳非 0、錯誤訊息⛔ 不帶機密，而且在 `down` 之前就停下）：

1. `set -euo pipefail`。Redis 白名單那段已經有明確的錯誤處理，保留原樣（它用 `if …; then rc=0; else rc=$?; fi`，不受 `set -e` 影響）。關鍵步驟一律**明確檢查結束碼**，⛔ 不只依賴 `set -e` 的隱含行為。
2. 路徑：
   * `SCRIPT_DIR` 由腳本本身的位置決定（`cd "$(dirname "${BASH_SOURCE[0]}")" && pwd`）；
   * `PROJECT_DIR` 是 build 用的 git checkout，跟 scripts 目錄分開，由使用者填寫；
   * `STACK_COMPOSE_FILE="$SCRIPT_DIR/compose.yml"`；
   * `APP_ENV_FILE="$SCRIPT_DIR/.env"`（只給 preflight 檢查用，必須跟 compose 檔裡寫的路徑相同）；
   * `REDIS_ENV_FILE`、`DEPLOY_BRANCH` 由使用者填寫。
3. preflight（都在 `git pull` 之前）：
   * `PROJECT_DIR` 是 git checkout，而且沒有本機修改（`git -C "$PROJECT_DIR" status --porcelain` 是空的）；
   * `STACK_COMPOSE_FILE` 存在；
   * `APP_ENV_FILE` 是一般檔案、可讀、權限 `600`；
   * compose 檔裡兩個 `env_file` 的路徑，跟 `APP_ENV_FILE`、`REDIS_ENV_FILE` 一致（用 awk 讀 compose 檔，只比對路徑字串）。
4. Redis 白名單區塊：照舊檢查 Redis stack 的 `.env` 只有一行 `REDIS_PASSWORD`，但⛔ 不再 `export`（compose 自己會讀，D7）。
5. `git -C "$PROJECT_DIR" pull --ff-only origin "$DEPLOY_BRANCH"`，只允許 fast-forward。
6. 設定檢查（D5）：用跟部署相同的參數跑 `docker compose … config`，**在記憶體裡**用 awk 取出 backend 最終的 `AUTH_JWT_SECRET`，檢查不是空值、而且⛔ 不是已知的公開範例值（`change-me-in-production`、`change-me`、`dev-only-change-me`）。只輸出「通過／失敗」，⛔ 不把 config 寫到檔案或印到畫面。`config` 本身失敗也算失敗。
   * 用 awk 而⛔ 不用 jq／python3：live 主機上有沒有這兩個工具還沒確認，awk 在 live 上已經實際用過。
   * 這一步只涵蓋 JWT，看的是 compose 解析後的設定，跟 image 無關，所以在 build 之前就能先擋。完整的契約由第 8 步用新 image 檢查。
7. **build（D11）**：`docker compose … build`，一次 build 一個服務（跟 `smoke-dev.sh` 一樣，避免兩個 image 的記憶體峰值疊加）。這時舊的服務**還在跑**。
   * ⚠️ 記憶體：這台主機約 2GiB，`smoke-dev.sh` 的註解記錄過冷 cache build 的可用記憶體低點只剩 74 MiB，所以 dev 是先停服務再 build。live 改成「不停機 build」之後，build 和正在跑的 live 服務會同時佔記憶體。對策：build 之前讀 `/proc/meminfo` 的 `MemAvailable`，低於門檻（實作時依 dev 的實測訂，記在腳本常數）就**停下來、⛔ 不 build、⛔ 不 down**，回報失敗，由人選擇改在低負載時段、或改用「先 down 再 build」的舊順序（D11 的退路）。build 失敗也一樣停下，舊服務照常運作。
8. **用新 image 檢查完整設定（D11、E）**：`docker compose … run --rm --no-deps backend ./trading-backend -check-config`（旗標名稱實作時定，這裡先用 `-check-config`）。
   * ⚠️ 一定要**寫出執行檔**：backend 的 Dockerfile 只有 `CMD ["./trading-backend"]`、沒有 `ENTRYPOINT`，compose 也沒有設定 `command:`。`compose run` 後面接的參數會整個取代 CMD，只寫旗標的話 container 會把旗標當成程式去執行，檢查根本跑不起來（v3 review）。工作目錄是 image 的 `WORKDIR /app`，`config.yaml` 就在那裡，跟正式啟動時一樣。
   * ⛔ 不改 Dockerfile 加 `ENTRYPOINT`：那會改變其他用 `run`／`exec` 帶指令的既有用法，範圍超出本案。它用的是剛 build 好的 image、跟正式啟動相同的 `env_file`／`environment`，只載入設定、跑契約檢查，⛔ 不連 DB 或任何外部服務。結束碼不是 0 就停下，⛔ 不 down。輸出只有鍵名和規則代號。
9. `docker compose … down`，結束碼不是 0 就停下，⛔ 不進 `up`。
10. `docker compose … up -d --no-build`：用第 7 步 build 好的 image，⛔ 不在停機後才 build。結束碼不是 0 就回報失敗。
11. readiness 確認：最多等 90 秒，期間從 container 內部打 backend 的 `/health`（backend image 是 alpine，有 busybox 的 `wget`）。要同時符合「拿到成功回應」和「backend 的 `RestartCount` 是 0」才算通過。逾時，或 container 處在 `Restarting`，都回報失敗。
   * ⚠️ `up -d` 成功只代表 Docker 收下了啟動要求，⛔ 不代表 backend 通過了設定和 Redis 的檢查。
   * 部署失敗時⛔ 沒有自動回滾：只會回報失敗，由人判斷要不要換回前一版的 image。

**D11 不採用時的退路**：第 7、8 步拿掉，第 10 步改回 `up --build -d`（先 down 再 build）。這樣 build 失敗、或第 6 步涵蓋不到的設定錯誤，都要到停機之後才會發現。測試要依最後採用的順序寫（第七節）。

### 4.4 C：啟動時的 Redis 探測

| 情況 | 現在 | 改成 |
|---|---|---|
| ⛔ 沒有設定 `REDIS_ADDR` | 記 `redis disabled (no addr configured)`，停用 | 不變 |
| 有設定、第一次就連上 | `redis connected` | 不變（另外記嘗試次數） |
| 有設定、連不上（`NOAUTH`、`WRONGPASS`、逾時、拒絕連線、DNS…） | 記 `redis connect failed, cache disabled`，**靜默停用** | 依 D3 重試，仍然失敗就記 Error `redis connect failed after retries`，帶上**封閉值域的原因類別**和嘗試次數，⛔ 不帶錯誤原文、密碼或位址以外的設定，然後 `os.Exit(1)` |

**探測的做法**：

* 另外建一個**啟動專用**的 client，只用來 ping：`MaxRetries: -1`（關掉 client 內建的重試）、`DialTimeout`／`ReadTimeout`／`WriteTimeout` 都設 3 秒、`PoolSize: 1`、`PoolTimeout` 3 秒、`ContextTimeoutEnabled: true`。每次嘗試再包一個 3 秒的 context。
* 每次嘗試都新建 client，用完立刻 `Close()`。成功之後，照舊用 `NewRedis` 建立 runtime 的 client。所以 runtime client 的選項和重連行為**完全不變**。
* 可以測試的核心函式：輸入是 ping 函式、嘗試次數、間隔，以及一個可以注入的等待函式（測試時不用真的 sleep）；輸出是成功（附嘗試次數），或者帶原因類別的錯誤。父 context 被取消時立刻停止。`main.go` 只負責記 log 和結束。
* 原因類別（封閉值域）：`noauth`、`wrongpass`、`timeout`、`refused`、`dns`、`other`。分類只看錯誤的型別和 Redis 錯誤碼的前綴，⛔ 不把錯誤原文寫進 log。
* 結束之後，`restart: unless-stopped` 會讓 container 反覆重啟，`docker ps` 看得到 `Restarting`，部署腳本的 readiness 也會失敗。失敗從「靜默」變成「明顯」。
* **contract 的變化**：以前「有設定位址」不代表 Redis 一定要能用，現在變成「有設定位址，Redis 就是必要的」。沒設定位址的情況不變。

### 4.5 手動操作 compose 的標準指令（F3）

v3 同日改（D7、D10）：compose 檔裡已經沒有必填的 `${}`，手動執行 `docker compose ps`／`logs` 不必先帶入任何東西。操作文件只要寫清楚：一律加上 `--project-directory` 指向 build 目錄（F4，build context 才會對）；⛔ 不要用 `config`，它會把機密整份印出來。

**live 先做的版本（2026-11-13，使用者要求）**：scripts 目錄的 `dc.sh`。用跟 `deploy.sh` 相同的白名單讀 `REDIS_PASSWORD`（⛔ 不印值），用跟 `deploy.sh` 相同的 compose 參數執行。子指令的規則（使用者選定）：`ps`、`logs`、`top`、`images`、`events`、`version` 直接執行；`config` 一律擋下（會印出機密）；其他子指令要加 `--yes`；compose 的全域選項不開放。腳本 SHA-256 `9d878e41…`，在 dev 用假的 `docker` 測過放行、擋下和讀密碼失敗的情境。repo 的範本照使用者的決定併入本案，等 v3 確認後一起實作。⚠️ D7 實施之後，`dc.sh` 不再需要讀 Redis 密碼；它剩下的用途是擋下 `config`、要求會改變狀態的子指令加 `--yes`、固定帶 `--project-directory`。範本版會拿掉讀密碼那段。

### 4.6 E：設定契約檢查

**位置**：`config.Load()` 之後、連 DB 之前。不通過就記一筆 Error `config contract violated`，帶上違反的鍵名和規則代號的清單，⛔ 不帶值，然後 `os.Exit(1)`。這時連 DB、migration、Redis、排程器、HTTP 都還沒開始。

**規則的類型**（實作第一步：逐鍵盤點 `config.go` 的每一個欄位，填進契約表，跟 v3 一起 review）：

| 類型 | 規則 | 例子（盤點時要逐一確認） |
|---|---|---|
| 必要 | 不能是空值 | `DATABASE_DSN`（production 時）、`AUTH_JWT_SECRET` |
| 不能是公開範例值 | 不能等於 repo 裡任何一處寫過的範例值（`config.yaml`、`deploy.sh` 範本、dev compose、`.env.example`）。**每一條都要寫明適用條件**：只有在這個值會被用到時才檢查；功能停用時，空值或範例值都放行（viper 會把空值退回 `config.yaml` 的範例值，例如 `fugle.api_key` 的 `YOUR_FUGLE_API_KEY`） | `AUTH_JWT_SECRET` ≠ `change-me-in-production`／`change-me`／`dev-only-change-me`（無條件：API 一定會用到）；`FINMIND_API_KEY` ≠ `YOUR_FINMIND_API_KEY`／`your-finmind-api-key`（條件：盤點時確認有沒有無條件註冊的工作用到 FinMind）；`FUGLE_API_KEY` ≠ `YOUR_FUGLE_API_KEY`／`your-fugle-api-key`（條件：`FUGLE_ENABLED=true`）；`DATABASE_DSN` 的帳密 ≠ `trading:trading`／`user:password`／`root:password`、密碼 ≠ `trading`／`password`；`REDIS_PASSWORD` ≠ `dev-redis-only`（條件：`REDIS_ADDR` 有值） |
| 條件必要 | 某個設定成立時，另一個就必要 | 有 `REDIS_ADDR` → `REDIS_PASSWORD` 不能是空值；`FUGLE_ENABLED=true` → `FUGLE_API_KEY` 必要；FinMind 有被無條件註冊的工作用到 → production 的 `FINMIND_API_KEY` 必要（盤點時確認哪些工作會用到） |
| 格式 | 解析得了 | cron 字串、`JOB_RETRY_CUTOFF` 這類時間、數值的合法範圍。已經由 scheduler 的正規化函式把關的，盤點時決定是搬進契約，還是維持原狀並在契約表註明 |

* **環境的區分（D9）**：dev 和測試要能用範例值啟動（dev compose 的 JWT 就是 `dev-only-change-me`），所以「不能是公開範例值」只在 production 生效。怎麼判斷是不是 production，見 D9。
* 範例值的清單寫在程式裡的一個常數，並有測試檢查：repo 裡出現的每個範例值都在清單內（避免以後新增範例值卻漏了）。**完整清單**（v3 review 補齊；2026-11-13 掃 `docker-compose*.yml`、`deploy.sh`、兩份 `config.yaml`）：JWT `change-me-in-production`、`change-me`、`dev-only-change-me`；FinMind `YOUR_FINMIND_API_KEY`、`your-finmind-api-key`；Fugle `YOUR_FUGLE_API_KEY`、`your-fugle-api-key`；DB 帳密 `trading:trading`、`user:password`、`root:password`，密碼 `trading`、`password`；Redis `dev-redis-only`。**刻意排除的例外：無**。測試的掃描範圍要涵蓋這些檔案，新增範例值時清單跟著補。
* 契約表的每一列都要有「適用條件」一欄（無條件，或某個開關是 true），「必要」和「不能是公開範例值」兩類都一樣。停用的選填功能⛔ 不能讓 backend 起不來。
* log 和錯誤訊息只有鍵名和規則代號（例如 `AUTH_JWT_SECRET:public_example`、`REDIS_PASSWORD:required_when_redis_addr`），⛔ 不印值，也⛔ 不印值的長度或前幾碼。
* **只檢查設定的模式（D11）**：backend 加一個旗標，只載入設定、跑契約檢查，然後以 0 或 1 結束，⛔ 不連 DB 或任何外部服務。部署腳本可以在 `down` 之前，用新 build 的 image 跑一次，整份契約不過就不停機。
* 跟 C 的分工：E 檢查「有沒有、是不是範例值」；C 檢查「實際連不連得上」。密碼帶錯（`WRONGPASS`）只有 C 抓得到。
* **載入失敗的輸出**（v3 review 補）：`config.Load()` 失敗時，⛔ 不再把原始錯誤寫進 log。改成回傳一個只帶安全資訊的錯誤：
  * 讀不到或解析不了設定檔 → `config_file:unreadable`／`config_file:invalid`（⛔ 不帶檔案內容或 YAML 錯誤的原文）；
  * 型別轉換失敗（mapstructure 的 `*mapstructure.Error`）→ 從錯誤裡只取出**鍵名**，轉成環境變數名稱，代號 `invalid_type`（例如 `JOB_RETRY_ENABLED:invalid_type`）；取不到鍵名時只記 `config:invalid_type`；
  * 正常啟動與 `-check-config` 都走同一個轉換，輸出格式跟契約違反相同（鍵名＋代號）。

## 五、決定事項

| 編號 | 問題 | 狀態與建議 |
|---|---|---|
| D1 | repo 的 compose 範本要同步到什麼程度？ | **建議（待確認）**：這次只同步 backend 的應用設定 `env_file`、`.env.example` 和部署範本（A1～A3）。DB 憑證檔、資源限制、log 掛載、external networks 這些主機專屬的結構另外開 todo。⛔ 不把主機的絕對路徑直接提交成範本 |
| D2 | `.env` 怎麼確定讀得到 | ✅ **使用者 2026-11-13 決定**：在 compose 裡，backend 用 `env_file:` 讀 `.env`（`required: true`，⛔ 不加 `format: raw`）；`environment:` 裡⛔ 不寫同名的鍵。live 和範本都用絕對路徑（範本寫佔位，4.1、D10）。v1 原本的方案是 `deploy.sh` 加 `--env-file`，已經作廢：那只能保證 `deploy.sh` 這一條路徑讀得到，手動執行時還是可能讀錯 |
| D3 | C 的重試參數 | **建議（待確認）**：**總共 5 次嘗試（含第一次）**，失敗後間隔 2 秒，最後一次失敗之後⛔ 不再等；單次探測最多 3 秒（4.4 的選項）。理論上限是 5 × 3 + 4 × 2 = 23 秒。寫成常數，⛔ 不開設定鍵。實際時間要在 dev 實測（第七節） |
| D4 | `NOAUTH`／`WRONGPASS` 這種重試也不會好的錯誤要不要跳過重試 | **建議（待確認）**：**一律走同一個有限次數的流程**，邏輯比較單純，最多多花約 20 秒。原因類別和嘗試次數都會記下來 |
| D5 | backend 用公開的預設 JWT 也能靜默啟動 | **v3 同日改**：完整政策由 E（D8）在本案處理，⛔ 不另外開 issue。部署範本 preflight 的 JWT 檢查保留（4.3 第 6 步），用途是在 `down` 之前擋下。⛔ 不把 JWT 的 `${…:?}` 放回 `environment:`（那會重新引入 D2 的覆蓋問題） |
| D7 | `REDIS_PASSWORD` 怎麼進 backend | ✅ **使用者 2026-11-13 決定**：compose 的 `env_file` 直接讀 Redis stack 的 `.env`，`environment:` 刪掉 `REDIS_PASSWORD`。live 現在就改（第八節第 1b 步）。代價是少了 `:?`：在 E 上線之前，手動執行 compose 時如果 Redis 的 `.env` 少了密碼，backend 會回到靜默停用 Redis；`deploy.sh` 的路徑仍有白名單檢查 |
| D8 | 程式裡檢查設定要件的範圍 | ✅ **使用者 2026-11-13 決定**：**完整的設定契約**（4.6）。實作第一步是逐鍵盤點，契約表跟 v3 一起 review |
| D9 | 怎麼判斷是不是 production（決定「範例值」規則要不要生效） | **建議（待確認）**：新增一個環境類別的設定鍵（例如 `APP_ENV`），值是 `production`／`dev`／`test`；**沒設定時當成 production**（最嚴格），所以 live 忘了設也是安全的一邊。dev compose 和測試明確設成 `dev`／`test`；在本機直接 `go run` 時要自己設，不然會被擋下，錯誤訊息要說清楚怎麼設 |
| D10 | 範本的 `env_file` 路徑怎麼寫 | **建議（待確認）**：寫成 `/path/to/…` 的絕對路徑佔位，複製時改成實際路徑（跟 `deploy.sh` 範本同一套慣例）。⛔ 不用 `${DEPLOY_ENV_FILE:?}`（手動指令又得帶變數）；⛔ 不能用相對路徑（2.3.1） |
| D11 | 部署前先用新 image 跑一次設定檢查 | **建議（待確認）**：`deploy.sh` 改成 `git pull` → 記憶體檢查 → `build`（逐一服務）→ 用新 image 跑只檢查設定的模式 → `down` → `up -d --no-build`（4.3 第 7～10 步）。設定不過、build 失敗都⛔ 不停機；停機時間也會縮短。代價：live 服務還在跑時 build，2GiB 的主機有 OOM 風險，所以 build 前檢查 `MemAvailable`，不夠就停下（退路是舊順序） |
| D6 | 部署流程的失敗即停 | **建議（待確認）**：**一起修**。`cd`、`git pull`、設定檢查任何一步失敗，都⛔ 不進 `down`／`up`；`down` 失敗也⛔ 不進 `up`。範本和 live 都要做（第八節第 2 步） |

## 六、風險與回滾

| 風險 | 對策 |
|---|---|
| C：Redis 真的掛掉時，backend 也起不來，以前至少還能降級運作 | 這是使用者選的取捨（2026-11-13：「重試幾次後退出」）。回滾方式是換回舊的 image |
| C：反覆重啟期間，每次啟動都會連 DB、檢查 migration | migration 是冪等的；Docker 的重啟間隔會逐步拉長。狀況會在 `docker ps` 和部署腳本的 readiness 直接看到 |
| C：探測的選項設錯，導致 runtime 行為也被改到 | 探測用獨立的 client，用完就關；runtime client 照舊用 `NewRedis`。測試要確認 `NewRedis` 的選項沒有變 |
| A2／A3：範本的 `env_file` 佔位路徑忘了改，或指到錯的檔 | `required: true` 讓 compose 當場報錯；`deploy.sh` 的 preflight 比對 compose 檔裡的路徑和實際檔案、檢查權限 |
| D7：拿掉 `:?` 之後，Redis 的 `.env` 少了密碼也能啟動 | E 上線之前：`deploy.sh` 路徑有白名單檢查，手動路徑只有 `required: true`（只保證檔案存在）。E 上線之後：契約檢查擋下 |
| E：契約太嚴，把 live 正常的設定擋下，backend 起不來 | 契約表逐鍵 review；上 live 之前，用 D11 的模式拿 live 的設定在 `down` 之前先跑一次；回滾是換回舊的 image |
| E：D9 的預設是 production，開發者在本機直接執行時被擋 | 錯誤訊息說明怎麼設環境類別；dev compose 和測試都明確設定 |
| A3：`set -e` 改變了既有的錯誤行為 | Redis 白名單那段已經用 `if` 包起來；其他關鍵步驟明確檢查結束碼。測試涵蓋每一種失敗情境 |
| D11：不停機 build 時記憶體不夠，OOM 砍掉 live 的服務 | build 前檢查 `MemAvailable`，低於門檻就停下、⛔ 不 build；一次 build 一個服務；門檻依 dev 實測訂。仍有疑慮就用退路（先 down 再 build） |
| A3：readiness 等太久或誤判 | 上限 90 秒；只看 `/health` 和 `RestartCount`。誤判的情況只會回報失敗，⛔ 不會自動回滾 |
| F3：手動 compose 指令都要先帶入密碼 | D7 實施之後就不用了（4.5） |
| `.env.example` 被誤當成 `.env` 用，或有人把機密寫進去 | 機密一律留空；開頭的說明寫清楚；`.gitignore` 已經排除 `.env` |

## 七、測試與驗證

* **C 的單元測試**（`backend/scripts/test.sh`）：
  * 沒設定位址 → 停用，⛔ 不 ping；
  * 第一次就成功；第 k 次才成功；
  * 全部失敗 → 回傳錯誤，嘗試次數是 5、等待了 4 次（用注入的等待函式記錄）；最後一次失敗之後⛔ 沒有再等；
  * 原因類別正確（`noauth`、`wrongpass`、`timeout`、`refused`、`dns`、`other`），錯誤裡⛔ 沒有密碼或錯誤原文；
  * 父 context 取消時立刻停止；
  * `NewRedis` 的選項沒有變（runtime 的 contract）。
* **E 的單元測試**（`backend/scripts/test.sh`）：
  * 契約表的每一條規則各有通過和違反的案例；條件必要的規則要測「條件不成立時不要求」；
  * 環境類別：production（含沒設定）時範例值被擋，dev／test 時放行；
  * 適用條件：production 下，`FUGLE_ENABLED=false` 而且 `FUGLE_API_KEY` 是空值（viper 退回 `YOUR_FUGLE_API_KEY`）→ **通過**；`FUGLE_ENABLED=true` 而 key 是空值或範例值 → 違反。其他有開關的功能都比照，各至少一組；
  * 違反清單只有鍵名和規則代號，用一個含特殊字元的假機密確認輸出裡⛔ 沒有值、長度或片段；
  * 公開範例值：清單裡的**每一個**值都要有案例——production 擋下、dev／test 放行（DSN 的 `trading:trading`、`user:password`、`root:password`、只有密碼 `trading`；Redis 的 `dev-redis-only`；JWT、FinMind、Fugle 各值）；DSN 的帳密不是範例值時通過；解析不了的 DSN 判 `invalid_value`；
  * **載入失敗**：型別錯誤（例如 `JOB_RETRY_ENABLED` 設成一個含特殊字元的假值）、設定檔不存在、設定檔 YAML 壞掉，三種都要檢查：回傳的錯誤型別是安全的那一種、內容只有鍵名和代號，而且**正常啟動的 log 與 `-check-config` 的輸出**都⛔ 不含那個假值、⛔ 不含檔案內容。
  * 範例值清單涵蓋 repo 裡出現過的每一個範例值（掃描 `config.yaml`、`deploy.sh`、compose 檔、`.env.example`）。
* **E 的 dev 實測**：用 `OVERRIDE` 讓 dev 的 backend 缺一個必要的鍵 → 以非 0 結束，log 只有 `config contract violated` 和鍵名，⛔ 沒有 `database connected`（代表在連 DB 之前就停了）；只檢查設定的模式用跟 4.3 第 8 步**完全相同的指令形式**（`docker compose … run --rm --no-deps backend ./trading-backend -check-config`）實際跑一次：設定齊全時回 0、缺鍵時回 1，而且不連 DB（postgres 停著也要能跑完）。
* **C 的 dev 實測**（`docker-compose.dev.yml`，用 `OVERRIDE`）：
  * 正常：`smoke-dev.sh` 通過，log 有 `redis connected`；
  * 密碼錯誤：backend 以非 0 結束，log 有 `redis connect failed after retries` 和 `noauth` 或 `wrongpass`，container 反覆重啟；
  * 沒有位址：`redis disabled`，服務正常；
  * 連不到的位址（例如沒有回應的 IP）：量測從啟動到結束的時間，要在 23 秒左右，而且⛔ 不能明顯超過；
  * **失敗時確實沒有啟動 HTTP 和排程器**：`/health` 一直沒有回應；log 沒有排程器啟動和 `AbortRunning` 的紀錄；`job_runs` 沒有新增。⛔ 不能只測函式的回傳值。
* **部署腳本的測試**（新增 `scripts/test-deploy-sh.sh`）：
  * 用假的 `git`／`docker` 放在 `PATH` 前面，記錄呼叫順序。驗證這些情況都⛔ 沒有呼叫 `down`／`up`，而且腳本回傳非 0：目錄錯誤、checkout 有本機修改、`git pull` 失敗、`.env` 不存在、權限不是 600、`config` 失敗、JWT 是空值或公開範例值。
  * build 失敗、`MemAvailable` 低於門檻、新 image 的設定檢查失敗 → ⛔ 沒有 `down`／`up`，回傳非 0（舊服務照常運作）；
  * `down` 失敗 → ⛔ 沒有 `up`、回傳非 0；`up` 失敗、readiness 逾時、`RestartCount` 不是 0 → 回傳非 0；
  * 正常情況 → 呼叫順序是 preflight → `git pull --ff-only` → `config`（JWT）→ 記憶體檢查 → `build`（逐一服務）→ `run --rm --no-deps backend ./trading-backend -check-config`（只檢查設定）→ `down` → `up -d --no-build` → readiness。`up` ⛔ 不能帶 `--build`；假 `docker` 要檢查 `run` 的參數裡**有寫執行檔**，只帶旗標就判失敗；
  * D11 不採用時改測退路的順序（`down` → `up --build -d`），兩種⛔ 不混用。
* **設定解析的測試**（同一支腳本，用真的 `docker compose config`，⛔ 不啟動任何服務）：
  * 用一份合成的設定檔，分別從 repo 目錄、scripts 目錄和其他目錄執行，project directory 都指向 build 目錄，backend 最終的設定都要相同。比的是解析後的內容，⛔ 不只檢查檔案存在。
  * `env_file` 指到不存在的檔 → 報錯；compose 檔裡的路徑跟 `APP_ENV_FILE`／`REDIS_ENV_FILE` 不一致 → preflight 報錯；
  * compose 檔裡⛔ 沒有任何必填的 `${…:?}`（手動指令不必帶變數，D7、D10）；`environment:` 裡⛔ 沒有 `REDIS_PASSWORD`。
  * `.env.example` 的鍵跟 backend `environment:` 的鍵⛔ 不能重複；`.env.example` 跟原本 `deploy.sh` 的 `export` 清單逐鍵比對，⛔ 不能漏鍵。
  * 值裡有 `$`、`#`、引號的寫法，解析結果要跟 4.2 的說明一致。
  * 輸出只有「通過／失敗」和鍵名。⛔ 不印完整的 `config`、DSN、JWT、API key；路徑或設定檢查失敗時，也⛔ 不用機密值當診斷資訊。
* **其他靜態檢查**：`bash -n deploy.sh`；有裝 shellcheck 的話也跑。

## 八、live 的同步（使用者執行；Claude 給指令和預期輸出）

1. ✅ **F1 修正（2026-11-13 完成）**：
   * F2 已改權限；E1 確認 build 目錄沒有 `.env`；A/B 實測確認了問題。
   * **`compose.yml`**：依 D2 改成用 `env_file` 讀 `.env`。先核對原檔 sha（`affec70c…`）和 `.env` 的 19 個鍵名，再比對修改前（加 `--env-file`）和修改後（不加，照 `deploy.sh` 的方式）的 `config`：backend 最終的 environment 一致（49 行，hash `a4ae0770475a`），確認後才寫入。寫入後再用 `deploy.sh` 的方式和在 scripts 目錄手動執行的方式各驗一次，都一致，`is not set` 警告 0 則。`diff` 只刪了那 19 行，另外加了 `.env` 的 `env_file` 和 2 行註解。新檔 SHA-256 `20825bfc9e5b6315fe6f209d4bca7a59579ee0120ad8b996db544766aa99b247`，備份是 `compose.yml.bak.20261113`。
   * **`deploy.sh`**：先核對原檔 sha（`5d569e06…`），在 `cd $PROJECT_DIR` 前面插入 13 行（2 行註解、讀不到的檢查、權限 600 的檢查、1 行空行）。寫入前把檢查區塊單獨試跑三種情況：真正的 `.env` 結束碼 0，權限 644 的假檔和不存在的檔都是 1。新檔 SHA-256 `07ebcc55c26bf82bde0a2cb23af2a4bb767c3f78d6f8da94b869e8b7d07d4838`（跟 Claude 在 dev 用同一份原檔預先算出的值一致），備份是 `deploy.sh.bak.20261113`。稍早給過一份「`deploy.sh` 加 `--env-file`」的指令，沒有執行，已經作廢。
   * **第一次用新設定部署（14:34 起）**：先確認 build 目錄的 `abdc24d` 跟 `origin/init` 只差 `e3e9156`，而且那個 commit 只改了文件；build 目錄沒有本機修改；`job_runs` 的 `running` 是 0。`./deploy.sh` 結束碼 0，backend 在 14:35:15 啟動。重建前後 backend 的 `Config.Env` 完全一致（50 行，hash `34d11e524211`），比對時⛔ 沒有印出任何值。log 有 `redis connected`、`yahoo intraday enabled`。**F1 在 live 結案。**
1b. ✅ **D7：live 的 `compose.yml` 改成用 `env_file` 讀 Redis 的 `.env`（2026-11-13 完成）**：先核對 sha（`20825bfc…`）和 Redis `.env` 的白名單（只有一行 `REDIS_PASSWORD`），再比對修改前（shell 帶入密碼、照 `deploy.sh` 的方式）和修改後（shell ⛔ 沒有密碼）backend 的最終 environment：都是 49 行、hash `87d53bbce18b`，一致才寫入。寫入後在 shell 沒有密碼的情況下，用 `deploy.sh` 的方式和在 scripts 目錄手動執行各驗一次，都一致，`is not set` 警告 0 則；手動 `docker compose ps` 不帶密碼也成功（三個服務都是 running，結束碼 0）。`diff` 只刪了 `REDIS_PASSWORD` 那 1 行，加了 Redis `.env` 的 `env_file`（`required: true`）和 2 行註解。新檔 SHA-256 `f75a32bdf1081c06bba3462671662cbcd97b442be0dd8a3a2bc38e51cb0b34b8`（跟 Claude 在 dev 預先算出的值一致），備份是 `compose.yml.bak.20261113b`。container ⛔ 沒有重建，下次 `up` 才套用。**F3 在 live 解決。**
2. **review 通過之後，live 的 `deploy.sh` 補上 D5、D6**：失敗即停（`set -euo pipefail`、`git -C`、`git pull --ff-only`、`down` 失敗不進 `up`）、JWT 的最小檢查、readiness 確認。live 的路徑和 compose 檔照現況（固定的 `.env` 絕對路徑、`./compose.yml`、build 目錄當 project directory），⛔ 不整份換成範本。做法跟這次一樣：先核對 sha，在 dev 用同一份原檔預先算出新檔的 sha，指令用 base64 加 sha 核對交付；寫入前單獨試跑每一個檢查。
3. **C 的部署**：要 build 新的 image 才會生效。依 `development-workflow.md` 的部署規則找時段，避開排程前後 10 分鐘，⛔ 不在早班批次到 08:45 之間。
   * ~~跟 I-131 協調：如果 C 在 I-131 正式合併之前部署，要記下 image 和設定的版本，I-131 的最終演練和驗收都要用同一個版本。~~ I-131 已在 2026-11-13 收斂（⛔ 沒有資料缺口、不需要合併），這項協調取消。
4. **部署後確認**：`redis connected`、readiness 通過、backend 的 `Config.Env` hash 跟部署前一致（沒有改設定時）。故意用錯的密碼⛔ 不在 live 上試，只在 dev 驗證。

## 九、完成後要歸檔的位置

* `docs/development-workflow.md`：
  * 「live 的 Redis」：backend 啟動時的新語意（有設定位址就必要；探測的次數與時間上限），以及 4.5 的標準載入指令；
  * 「只改 live 的環境變數」：改 scripts 目錄的 `.env`（`env_file` 注入，⛔ 不再是 `${}` 展開）；`environment:` 和 `.env` 不能有同名的鍵；
  * 「新增排程開關要改的地方」：`export` 改成 `.env.example`；
  * 部署腳本的流程：失敗即停、preflight、readiness，以及⛔ 沒有自動回滾；
  * Compose 的最低版本：`env_file` 的 `path`／`required` 寫法需要 Compose 2.24 以上（實作時以官方文件再確認）；live 和 dev 都是 v5.7.0。
* `docs/architecture.md`「生產 / Docker」：設定的來源（應用 `.env`／Redis 的 `.env`／DB 憑證的 `env_file`，都由 `env_file` 注入）、Redis 的啟動語意，以及**設定契約表**（E：每個鍵的規則、環境類別的語意、範例值清單在哪裡）。
* `docs/issue.md` I-132：改成「已實作／待 review」；review 之後收斂並刪除本計畫書（要先 commit 過）。

## 十、證據

| 編號 | 內容 | 取得方式 |
|---|---|---|
| E1 | build 目錄 `/opt/stacks/deploy/stock_trading/build/stock_trading/` 底下有沒有 `.env` | ✅ 2026-11-13 使用者查證：⛔ 沒有。scripts 目錄的 `.env` 雜湊開頭是 `06875c113f12`，權限已改成 600 |
| E2 | F1 的 A/B 實測、live 修改前後的設定比對、第一次部署前後的 `Config.Env` 比對 | ✅ 2026-11-13 使用者在 live 執行、Claude 核對輸出（2.3、第八節第 1 步） |
| E3 | go-redis v9.5.4 的預設選項和重試條件 | 實作時讀 module 原始碼確認（2.5），並用 dev 的計時實測佐證 |

## 附錄 C：設定契約表（E 的盤點，2026-11-13，待 review）

> 實作第一步的產出。依 `backend/internal/config/config.go` 的每個欄位、`config.Load()` 的 `SetDefault`、`backend/config.yaml`，以及 `main.go`／scheduler 實際怎麼用，逐鍵整理。**這張表 review 通過之後才寫程式。**
> 環境變數的名稱是 viper 的鍵把 `.` 換成 `_` 再轉大寫（例如 `auth.jwt_secret` → `AUTH_JWT_SECRET`）。
> 「prod」＝ D9 的環境類別是 production（含沒設定）；「全部」＝ production／dev／test 都檢查。

### C.1 盤點時查到、會影響規則的事實

* **viper 會把空字串當成沒設定**，退回 `config.yaml` 或 `SetDefault` 的值（2.3）。所以檢查的永遠是**最終值**；「空值」和「範例值」要一起擋。
* **`database.driver` 的預設是 `sqlite`、`dsn` 的預設是本機檔案路徑**。live 如果漏了 `DATABASE_DRIVER`，backend 會**靜默開一個全新的空 SQLite**，排程照跑、API 回空資料。這是盤點裡後果最嚴重的一項。
* **FinMind 是無條件用到的**：`main.go` 無條件建立 FinMind client，公司行動同步（無條件註冊）、籌碼採集（無條件註冊）、日 K 回補都用它。現在 key 是空值或 `YOUR_FINMIND_API_KEY` 時只記一筆 Warn。
* **Fugle 只在 `fugle.enabled=true` 時用到**；現在 key 是空值或 `YOUR_FUGLE_API_KEY` 時只記一筆 Warn。`config.yaml` 的預設值就是 `YOUR_FUGLE_API_KEY`。
* **`python.service_url` 空白時**，SR 分析（排程和手動）會在執行時才失敗（`python service url not configured`）。
* **cron 字串打錯時**，scheduler 的 `AddFunc` 只記 Error、不中止，那支排程就**靜默不註冊**。只有 `corporate_action.cron` 有退回預設值的第二道防線。
* **已經有正規化的設定**：`job_retry.*`（≤ 0 或格式錯誤退回預設並記 Error）、`candle_gap_detection.*`（scheduler 的正規化函式）、`sr_zone_verify.*`（兩段式）、`corporate_action.timeout_sec`／`shard_count`（≤ 0 退回預設）、`stock_symbols.timeout_sec`／`fetch_delay_sec`、`delisting.timeout_sec`／`cron`（0 或空白沿用預設）。
* ~~型別解析失敗（例如 `ENABLED=abc`）現在就會讓 `config.Load()` 失敗、啟動中止，契約⛔ 不重複處理。~~ **v3 review 修正**：型別解析失敗確實會讓啟動中止，但 `main.go` 把 `config.Load()` 的**原始錯誤**記進 log（`config load failed`）。mapstructure v1.5.0 的轉型錯誤會帶出輸入值（例如 `cannot parse 'job_retry.enabled' as bool: strconv.ParseBool: parsing "…"`），viper 讀設定檔失敗的錯誤也可能帶出檔案內容的片段。**所以載入失敗也要轉成安全的輸出**：見 4.6「載入失敗的輸出」。

### C.2 契約表

| 設定（環境變數） | 預設值 | 誰用、什麼時候用 | 規則 | 適用條件 | 環境 |
|---|---|---|---|---|---|
| `APP_ENV`（新增，D9） | 沒設定＝production | 決定下面「prod」的規則要不要生效 | 只能是 `production`／`dev`／`test`，其他值**一律擋下**（打錯字⛔ 不能默默變成寬鬆模式） | 無條件 | 全部 |
| `DATABASE_DRIVER` | `sqlite` | DB 連線 | 必須是 `postgres` | 無條件 | prod |
| `DATABASE_DSN` | `../../output/stock_trading/trading.db`（config.yaml） | DB 連線 | 不能是空值；⛔ 不能是預設的 SQLite 路徑；**帳密⛔ 不能是 repo 裡的公開範例**：解析 DSN 的 userinfo，使用者＋密碼是 `trading:trading`（`docker-compose.yml`、dev compose、`docker-compose.postgres.yml`）、`user:password`、`root:password`（兩份 `config.yaml` 的註解），或密碼是 `trading`（`docker-compose.mysql.yml`）、`password`，都擋下。解析不了的 DSN 判 `invalid_value` | 無條件 | prod |
| `SERVER_PORT` | `8080` | HTTP | 必須是 1～65535 的整數 | 無條件 | 全部 |
| `REDIS_ADDR` | 空白（＝停用） | 快取、訊號佇列 | ⛔ 不能是空值（live 一定要有 Redis）；dev／test 允許空白＝停用 | 無條件 | prod |
| `REDIS_PASSWORD` | 空白 | Redis 認證 | ⛔ 不能是空值；⛔ 不能是 repo 裡的公開範例 `dev-redis-only`（dev compose 的預設值） | `REDIS_ADDR` 有值 | prod（dev／test 用 `dev-redis-only` 放行） |
| `REDIS_DB` | `0` | Redis | 必須 ≥ 0 | `REDIS_ADDR` 有值 | 全部 |
| `AUTH_JWT_SECRET` | `change-me-in-production` | 登入 token 的簽章 | ⛔ 不能是空值；⛔ 不能是公開範例值（`change-me-in-production`、`change-me`、`dev-only-change-me`）；長度至少 32 bytes（Q1 ✅） | 無條件（API 一定會用到） | 不能是空值：全部；範例值與長度：prod |
| `FINMIND_API_KEY` | `YOUR_FINMIND_API_KEY` | 日 K、公司行動、籌碼（都無條件） | ⛔ 不能是空值或範例值（`YOUR_FINMIND_API_KEY`、`your-finmind-api-key`） | 無條件 | prod |
| `FINMIND_BASE_URL`、`FINMIND_RATE_LIMIT` | 官方網址、`5` | FinMind client | 網址要能解析成 `https`；rate limit ≥ 1 | 無條件 | 全部 |
| `FINMIND_INTRADAY_ENABLED` | `false` | 盤中分 K | （開關本身只有型別檢查） | — | — |
| `FUGLE_API_KEY` | `YOUR_FUGLE_API_KEY` | Fugle 即時行情 | ⛔ 不能是空值或範例值（`YOUR_FUGLE_API_KEY`、`your-fugle-api-key`） | **`FUGLE_ENABLED=true`**；停用時空值、範例值都放行 | prod |
| `FUGLE_*` 的網址和數值 | 見 `SetDefault` | Fugle | 網址要能解析；`QUOTE_RATE_LIMIT`、`MAX_SUBSCRIPTIONS` ≥ 1；其他維持現行（0／負值有既定語意，例如 `PING_INTERVAL_SEC` 負值＝關閉 ping） | `FUGLE_ENABLED=true` | 全部 |
| `YAHOO_BASE_URL`、`YAHOO_RATE_LIMIT`、`YAHOO_BATCH_SIZE` | 見 `SetDefault` | Yahoo 盤中；`RATE_LIMIT` 另外給除權息來源用（無條件） | 網址要能解析（`YAHOO_ENABLED=true` 時）；`RATE_LIMIT` ≥ 1（無條件，除權息會用）；`BATCH_SIZE` ≥ 1（啟用時） | 見規則 | 全部 |
| `PYTHON_SERVICE_URL` | 空白 | SR 分析（排程與手動）、回測的 Method B | ⛔ 不能是空值，而且要能解析成 `http(s)` 網址 | 無條件 | prod |
| `PYTHON_SR_ZONES_TIMEOUT_SEC` | `120` | SR zone 呼叫 | ≥ 1 | 無條件 | 全部 |
| `STOCK_SYMBOLS_CRON` | `30 6 * * *` | 主檔同步 | `cron.ParseStandard` 解析得了 | `STOCK_SYMBOLS_ENABLED=true` | 全部 |
| `STOCK_SYMBOLS_SYNC_URLS` | 上市＋上櫃兩個網址 | 主檔同步 | 至少一個、每個都能解析 | `STOCK_SYMBOLS_ENABLED=true` | 全部 |
| `CHIP_SYNC_CRON` | `0 21 * * 1-5` | 籌碼採集（無條件註冊） | 解析得了 | 無條件 | 全部 |
| `CHIP_SYNC_HISTORY_TRADING_DAYS`、`CHIP_SYNC_BATCH_SIZE` | `500`、`50` | 籌碼 | ≥ 1 | 無條件 | 全部 |
| `CORPORATE_ACTION_CRON` | `30 6 * * 1-5` | 公司行動（無條件註冊） | 解析得了。⚠️ 現在打錯會退回預設（第二道防線）；契約改成**啟動就擋**，scheduler 的退回邏輯保留給不經過 `config.Load()` 的呼叫端 | 無條件 | 全部 |
| `EVALUATION_UNIVERSE_CRON`、`_DAYS` | `0 16 * * 1-5`、`10` | 評估標的池 | cron 解析得了；`DAYS` ≥ 1 | `EVALUATION_UNIVERSE_ENABLED=true` | 全部 |
| `SR_ANALYSIS_CRON`、`_CHIP_CRON`、`_TIMEFRAME`、`_LIMIT` | `0 17 * * 1-5`、`0 22 * * 1-5`、`1d`、`400` | SR 分析排程 | 兩個 cron 都解析得了；`TIMEFRAME` 是程式認得的週期（實作時對照 API 的合法值）；`LIMIT` ≥ 1 | `SR_ANALYSIS_ENABLED=true` | 全部 |
| `SR_EVALUATION_CRON`、`_TIMEFRAME`、`_LIMIT`、`_REPLAY_MAX_ROWS` | `30 22 * * 1-5`、`1d`、`1500`、`200` | SR evaluation | cron 解析得了；`TIMEFRAME` 同上；兩個數值 ≥ 1 | `SR_EVALUATION_ENABLED=true` | 全部 |
| `DELISTING_URL`、`DELISTING_CRON` | 官方 CSV 網址、`0 7 * * *` | 終止上市對帳 | 網址要能解析；cron 空白沿用預設，有值就要解析得了 | `DELISTING_ENABLED=true` | 全部 |
| `JOB_RETRY_*` | 見 `SetDefault` | 自動重跑 | **維持現行的正規化**（退回預設＋Error），⛔ 不改成啟動失敗：那是 I-123～I-126 已確認的行為 | — | — |
| `CANDLE_GAP_DETECTION_*` | 見 `SetDefault` | 日 K 缺漏偵測 | **維持現行**：scheduler 的正規化函式把關（`config.go` 的註解寫明了分工） | — | — |
| `SR_ZONE_VERIFY_*` | `30`、`2000` | SR zone 驗證 | **維持現行**（兩段式） | — | — |
| `CORPORATE_ACTION_TIMEOUT_SEC`、`_SHARD_COUNT` | `2700`、`5` | 公司行動 | **維持現行**（≤ 0 退回預設） | — | — |
| `STOCK_SYMBOLS_TIMEOUT_SEC`、`_FETCH_DELAY_SEC`、`DELISTING_TIMEOUT_SEC` | `0`（＝沿用程式預設） | 各 client | **維持現行**（0 有既定語意）；負值擋下 | 對應功能啟用時 | 全部 |
| `POSITION_ANALYSIS_*` | 見 `SetDefault` | 持倉分析（`portfolio.NewAnalyzer`） | `MAX_POSITION_VALUE`、`MAX_RISK_AMOUNT`、`MIN_RISK_REWARD_RATIO`、`BREAKOUT_TARGET_RISK_REWARD_RATIO` > 0；`ADD_ON_RATIO`、`TAKE_PROFIT_REDUCTION_RATIO` 在 (0, 1]；`SR_REUSE_MAX_AGE_HOURS` > 0（Q2 ✅ 使用者決定納入）。⚠️ **行為改變**：現在 ≤ 0 時 `NewAnalyzer` 各自**靜默退回預設值**（⛔ 沒有 log），契約之後改成啟動就擋；比例的上限現在⛔ 沒有檢查，`TAKE_PROFIT_REDUCTION_RATIO` > 1 會讓減碼的目標股數變成負數。`NewAnalyzer` 的退回邏輯保留給不經過 `config.Load()` 的呼叫端 | 無條件 | 全部 |
| 各功能的 `*_ENABLED` 開關 | 見 `SetDefault` | — | 只有型別檢查（現行） | — | — |

**輸出的規則代號**（log 只寫「鍵名:代號」）：`invalid_type`（載入時的型別錯誤）、`config_file`（讀不到或解析不了設定檔，代號 `unreadable`／`invalid`）、`required`、`public_example`、`invalid_value`、`invalid_cron`、`invalid_url`、`out_of_range`、`too_short`、`required_when_<條件>`（例如 `REDIS_PASSWORD:required_when_redis_addr`）。

### C.3 待使用者確認

* ✅ **Q1（使用者 2026-11-13 決定）**：production 要求 `AUTH_JWT_SECRET` 至少 32 bytes。live 原本只有 27 bytes，**2026-11-13 17:40 已換成 64 個 hex 字元**（`/dev/urandom` 32 bytes；腳本以 base64 加 sha 交付、在 dev 用假的 `.env` 試跑過）：前置核對 `.env` 權限 600、JWT 剛好 1 行、`running` 0；只替換那一行、鍵數與其他行不變；舊檔備份成 `.env.bak.20261113174038`（600）；只重建 backend（`--no-build --no-deps`）。驗證：container 裡的長度 64、JWT 以外的環境變數重建前後一致（hash `159eaadd3842`）、log 有 `redis connected`、`job retry config`、`scheduler started`。舊 token 全部失效，要重新登入。⛔ 整個過程沒有印出任何值。備份檔含舊的 secret，確認登入正常之後由使用者刪除。
* ✅ **Q2（使用者 2026-11-13 決定：納入，照表裡的範圍）**：讀碼確認現在 ≤ 0 會靜默退回預設值、比例沒有上限檢查（見表）。`SR_REUSE_MAX_AGE_HOURS` 依讀碼結果從「≥ 0」改成「> 0」（0 現在代表用預設值）。
* ✅ **Q3（使用者 2026-11-13 決定）**：production 一定要有 `REDIS_ADDR`。將來有不用 Redis 的部署方式時再拿掉。
* ✅ **Q4（使用者 2026-11-13 決定）**：啟動就擋。scheduler 的退回邏輯保留給不經過 `config.Load()` 的呼叫端。

### C.4 上線前的核對

* 上 live 之前，用 D11 的「只檢查設定」模式，拿 live 的實際設定跑一次（在 `down` 之前），全部通過才部署。
* live 的 DB 密碼（`trading-go.env`）是自動產生的，⛔ 不會是 `trading` 這類範例值（使用者 2026-11-13 說明；部署前的 D11 檢查仍會再驗一次）。
* 目前 live 的設定（2.2）預期會通過：`DATABASE_DRIVER=postgres`、`REDIS_ADDR` 和密碼都有、`PYTHON_SERVICE_URL` 有值、`FUGLE_ENABLED` 預設 false、FinMind 和 JWT 都不是範例值（使用者 2026-11-13 比對過 JWT 跟 `.env` 一致；長度待 Q1）。
* live 要在 compose 加上 `APP_ENV`？D9 的設計是「沒設定＝production」，所以 live ⛔ 不需要加；dev compose 要加 `APP_ENV: dev`。

## Review feedback（2026-11-13，v2）

> 狀態：✅ 已在 v3 處理（對照見「十一、修訂紀錄」v3）。這一節保留原文供追溯。原始狀態：裁決建議／待納入 v3。本輪只 review 與評估，未修改範本、程式或 live。D2 已有使用者裁決；以下建議不冒充使用者已確認的決定。v2 的第三節、4.1、測試及歸檔段落仍含已撤回的 `--env-file` 方案，需先在 v3 同步後再實作。

### D1、D3～D5 與新增 D6 的建議

| 編號 | 建議 | 需要寫進 v3 的邊界 |
|---|---|---|
| D1 | 本次同步 backend 的應用設定 `env_file`、`.env.example` 和部署範本；不直接整份照抄 live compose | 配合已定案的 D2，不能只改 Redis 的 `:?` 而保留會覆蓋 `.env` 的同名 `environment`。DB 憑證檔、資源限制、log 掛載及其他主機專屬結構另列後續工作；不能把主機絕對路徑直接提交成通用範本 |
| D3 | **總共 5 次嘗試，包含第一次**；失敗間隔 2 秒，單次啟動探測最多 3 秒 | 最後一次失敗後不再 sleep；理論探測加等待為 5 × 3 ＋ 4 × 2＝23 秒，約 25 秒僅作含排程開銷的估計，須測實際超時行為 |
| D4 | 同意 NOAUTH／WRONGPASS 也走同一個有限次數流程 | 記安全的封閉原因類別與嘗試次數，不印 Redis 原始錯誤、密碼或含憑證的設定；全敗時在 HTTP／scheduler 啟動前以非 0 結束 |
| D5 | 跨環境的 backend JWT 強制拒絕政策另案，但本次部署前至少拒絕空 JWT 與已知的公開範例值 | 這個最小檢查放在 production 部署範本的 preflight，不能只檢查 `.env` 存在和 mode 600；不藉此推斷或修改 dev 的既有認證模式。檢查解析後的最終 backend 設定，不把 JWT 的 `${…:?}` 放回 `environment` 而重引入 D2 的覆蓋問題。完整的 backend 設定契約另在 issue 追蹤，不能因「另案」而遺失 |
| D6（新增） | **一起修部署流程的失敗中止** | `cd`／git pull／設定校驗失敗，一律不能進入 `down`／`up`；`down` 失敗也不能繼續 `up`。不要只依賴 `set -e` 的隱含行為，關鍵步驟明確檢查結束碼 |

**D3 的實作風險（P2）**：repo 使用 go-redis v9.5.4，`NewRedis` 目前沒有設定重試與逾時選項。client 有自己的重試和 I/O timeout，context deadline 是否套用也受選項影響；只在外面包五次迴圈或傳入 3 秒 context，不足以證明單次上限。v3 要寫明如何限制啟動探測的內部重試、dial／read／write／pool 等待及 context deadline，並避免順便改變服務運行期間的重連契約。可採啟動專用探測，且明定資源關閉方式。依據：[go-redis v9.5.4 options](https://github.com/redis/go-redis/blob/v9.5.4/options.go)、[client 的 context 與重試處理](https://github.com/redis/go-redis/blob/v9.5.4/redis.go)。

### 4.1：repo 的 env_file 路徑參數化方向

建議用必填的絕對路徑參數 `DEPLOY_ENV_FILE`，不依賴 caller 的 CWD 或 build 目錄裡是否剛好有 `.env`：

```yaml
services:
  backend:
    env_file:
      - path: ${DEPLOY_ENV_FILE:?請明確設定應用設定檔的絕對路徑}
        required: true
```

* `deploy.sh` 由腳本本身的位置決定 `SCRIPT_DIR`，將 `DEPLOY_ENV_FILE` 明確設為該目錄的 `.env` 絕對路徑；檢查可讀、一般檔案及 mode 600。`PROJECT_DIR` 是 build checkout，與設定檔目錄分開。不從尚未讀取的應用 `env_file` 反過來取得自己的路徑。
* 手動 compose 操作也必須明確傳入同一個絕對路徑，例如 `DEPLOY_ENV_FILE=/absolute/scripts/stock_trading/.env docker compose …`；漏傳就拒絕執行，不能默默改讀別處。這是可攜範本的操作契約：參數化路徑不代表裸跑任意 compose 指令就能自行發現正確的主機路徑。live 已驗證的固定絕對路徑可維持，不必為了 repo 範本立刻改 live。
* `REDIS_PASSWORD` 繼續由既有白名單從 Redis stack 的 `.env` 讀入 shell，compose 保留 `:?`。應用 `env_file` 不作 compose `${…}` 的插值來源，所以不能把 Redis 密碼只放進應用 `env_file` 並期待 `:?` 通過。
* backend 的應用設定鍵搬到 `.env.example` 後，從 `environment` 刪除同名項目（包括 `${X:-false}` 這種預設），避免覆蓋 `env_file`。固定的服務連線設定與 Redis 密碼若保留在 `environment`，需列清單並避免同名。
* `.env.example` 的機密留空、開關保留 repo 的保守預設；不把 live 已開啟的開關照搬為範本預設。不用 shell `source` 解析 `.env`。格式說明應依 Compose 的解析規則，測試含 `$`、`#` 與引號的值，不沿用「任何值都不加引號」的籠統規則。
* 4.1 的檔名要一致：repo 目前是 `docker-compose.yml`，live 是 `compose.yml`。用明確的 `COMPOSE_FILE` 或對應範本的路徑，不能假設 repo 本來就有 `compose.yml`。新路徑參數、Compose 最低版本及手動操作方式一併歸檔。

依據：[Compose env_file 與 environment 優先順序](https://docs.docker.com/reference/compose-file/services/#env_file)、[Compose 插值來源](https://docs.docker.com/compose/how-tos/environment-variables/variable-interpolation/)。

### deploy.sh 失敗中止與部署驗證

live 的 `cd`／`git pull` 流程由本計畫既有紀錄確認；repo 現有 `deploy.sh` 則只有 `down`、`up`，也沒有整體失敗中止。v3 要明列要同步哪一種流程，不能混稱兩者已相同。

建議加入 `set -euo pipefail`，但保留 Redis 白名單現有的顯式錯誤處理；可用 `git -C "$PROJECT_DIR"` 避免 CWD 錯誤。更新原始碼與必要 preflight 必須在 `down` 之前完成。若加入 pull，建議只允許 fast-forward；任何失敗都回非 0，錯誤訊息不帶機密。

另外 C 上線後，`up -d` 成功只代表提交啟動要求，不能證明 backend 已通過 Redis 檢查；部署驗證必須等待 backend readiness，並在持續 Restarting／未 ready 時回報失敗。`set -e` 不能取代這個確認，也不表示部署具備自動回滾能力。

測試需包含：

* 用假 git／docker 命令記錄呼叫順序，驗證目錄錯誤、pull 失敗、env 檔缺失或權限錯誤、config 失敗時，都沒有 `down`／`up`；`down` 失敗不執行 `up`，任何失敗都不回成功。
* 相同合成設定檔，從 repo／scripts／其他 CWD 操作，並以 build 目錄當 project directory，解析後的 backend 設定必須相同；用內容斷言驗證，不只比較設定檔存在。缺路徑、缺檔及與 `environment` 同名的回歸情境要覆蓋。
* 驗證 compose 輸出時只在記憶體解析，只印必要的通過／失敗或鍵名，不輸出完整 `config`、DSN、JWT、API key。路徑／設定檔檢查不能用機密值當診斷資訊。
* C 的 targeted Go 測試依 repo script 執行；使用可注入的等待與 ping 做次數／取消測試，並在 isolated dev stack 測實際連線逾時、錯密碼與無位址。測真正非 0 退出及 API／scheduler 不會在失敗時啟動，避免只測 helper 的回傳值。

### 與 I-131 v7／D5(d) 的關係

I-131 的 R1～R4 仍待處理，不能以 I-132 已修復 live `.env` 或 Redis 連線來代替。I-131 D5(d) 建議具體化為「同一個 dev compose／project，使用作業專用 PostgreSQL volume」，保留原 dev volume；詳見 I-131 v7 的整合評估補充。I-132 的部署範本與 Redis 啟動 contract 仍需先整理為 v3 的可審查計畫；本 feedback 不直接把 v2 標成已確認。

## 十一、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-13 | 初版：A（A1 `:?`、A2 `.env.example`），C（重試之後非 0 結束），B 不做；記下 F1～F4 和範本落差。同日補充：E1 確認 build 目錄沒有 `.env`，查出 F1 的實際後果（viper 退回 `config.yaml` 的公開預設值），新增 D5；F2 已完成 |
| v2 | 2026-11-13 | A/B 實測確認 F1，也確認 `--env-file` 修得好；使用者決定 D2 改成在 compose 裡用 `env_file` 注入 `.env`，不在 `deploy.sh` 加 `--env-file`（見 F1 的處置、D2、第八節第 1 步）。4.1、A2、第三節受影響檔案裡的 `--env-file` 寫法和資料流描述還沒改，等 live 驗證完成後一起改寫 |
| v3 | 2026-11-13 | 依 v2 的 review 修正（逐項判斷：全部採納，D5 的做法有調整）：①D1 採建議：只同步 backend 的應用設定 `env_file`、`.env.example` 和部署範本，主機專屬結構另開 todo；②D2 落實到範本：`env_file` 的路徑用必填的 `DEPLOY_ENV_FILE`（4.1），live 維持已驗證的固定路徑；`environment:` 和 `.env.example` 的鍵不能重複，由測試擋；改正第三節、資料流、4.1、第九節裡已作廢的 `--env-file` 寫法；③D3 改正為「總共 5 次、最後一次失敗不再等、理論上限 23 秒」；[P2] go-redis 預設的重試與逾時會讓單次 ping 超過 3 秒 → 新增 2.5 和 4.4：用啟動專用的探測 client（`MaxRetries: -1`、逾時 3 秒、`ContextTimeoutEnabled`、用完就關），runtime client 不變；④D4 採建議，log ⛔ 不帶錯誤原文；⑤D5 採建議但調整做法：部署 preflight 用 awk 在記憶體裡檢查最終設定的 JWT（不用 jq／python3，因為 live 主機有沒有這兩個工具未確認），完整政策另開 issue；⑥新增 D6：失敗即停（`set -euo pipefail`、`git -C`、`pull --ff-only`、`down` 失敗不進 `up`），範本採用 live 的流程（新增 2.2 的流程差異表）；⑦readiness：`up -d` 成功不等於 backend 通過 Redis 檢查 → 4.3 第 9 步；⑧新增 2.4 啟動順序（Redis 失敗時 `AbortRunning`、排程器、HTTP 都還沒啟動）；⑨第七節補上假 `git`／`docker` 的呼叫順序測試、從不同目錄解析設定的比對、特殊字元的值、失敗時確實沒有啟動 HTTP 和排程器的 dev 實測；⑩第八節第 2 步：live 的 `deploy.sh` 也要補 D5、D6，做法同第 1 步；第 3 步記下 C 部署時要給 I-131 的版本資訊。**同日補充五（review）**：①[P2] 公開範例憑證補齊：DSN 的 `trading:trading`／`user:password`／`root:password`、密碼 `trading`／`password`，Redis 的 `dev-redis-only`，production 擋下、dev／test 放行，每個值都有案例；完整清單寫在 4.6，例外：無；②[P2] `config.Load()` 的原始錯誤會帶出輸入值（mapstructure v1.5.0 的轉型錯誤）→ 載入失敗也轉成鍵名＋代號（`invalid_type`、`config_file:*`），正常啟動與 `-check-config` 都測假值不外洩。**同日補充四**：E 的實作第一步完成——新增附錄 C 設定契約表（逐鍵盤點；發現 `DATABASE_DRIVER` 漏設會靜默開空的 SQLite、cron 打錯會靜默不註冊），C.3 列出 Q1～Q4 待確認。**同日補充三（review）**：[P2] D11 的設定檢查要寫出執行檔（`run --rm --no-deps backend ./trading-backend -check-config`）：Dockerfile 只有 CMD、沒有 ENTRYPOINT，`compose run` 的參數會整個取代 CMD；⛔ 不為此改 Dockerfile；測試要驗證指令形式，dev 實測用同一個形式、postgres 停著也要能跑完。**同日補充二（review）**：①[P2] 4.6 的「不能是公開範例值」每一條都要有適用條件，停用的選填功能（例如 `FUGLE_ENABLED=false`、key 留空時 viper 退回 `YOUR_FUGLE_API_KEY`）⛔ 不能讓 backend 起不來，補上測試；範例值清單補 `YOUR_FUGLE_API_KEY`；②[P2] 4.3 主流程和第七節的測試同步成 D11 的順序（build → 用新 image 檢查設定 → down → `up -d --no-build`），並寫出 D11 不採用時的退路；另外補上 review 沒提到的風險：2GiB 主機上不停機 build 可能 OOM → build 前檢查 `MemAvailable`、逐一服務 build。**同日補充（使用者決定）**：D7 `REDIS_PASSWORD` 改由 `env_file` 讀 Redis stack 的 `.env`，live 現在就改（第八節 1b）；D8 新增 E「設定契約檢查」，範圍是完整的契約（4.6），D5 併入 E、⛔ 不另開 issue；新增 D9（環境類別，沒設定當 production）、D10（`env_file` 用絕對路徑佔位，取代 `DEPLOY_ENV_FILE`；依 2.3.1 的實測，相對路徑是相對於 project directory）、D11（部署前用新 image 跑只檢查設定的模式）；4.5 的 `dc.sh` 記下 live 版本，D7 之後不再需要讀密碼 |
