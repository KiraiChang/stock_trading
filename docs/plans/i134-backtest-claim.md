# I-134：回測任務的原子認領與重啟後的遺留 `running`（計畫書 v1.1）

> 狀態：v1.1 待確認（v1 的 review 3 個 P2 與三項範圍追加已納入）。對應：`docs/issue.md` I-134。需求裁決：2026-11-18（D1～D4，見第二節）。
> 相關：`docs/todo.md` T-084（`deploy.sh` 自動檢查回測，D4 另列）、T-083（部署規範裡的回測雙零檢查）。

## 一、目標與不做的範圍

**目標**

1. **同一筆回測只執行一次**：python-worker 與 python-server 兩條執行路徑都保留（D3），但都改用同一套**原子認領**：一個 `UPDATE` 同時把狀態從 `pending` 改成 `running`、寫入執行者與開始時間，只有影響 1 列的一方能執行。
2. **重啟後收掉自己留下的 `running`**：每個 python 服務啟動時，把**自己認領、而且還在 `running`** 的任務改成 `failed`，原因 `interrupted_on_restart`，寫入 `finished_at`（D1）。⛔ 不自動重跑。
3. **結果、交易明細與 `done` 在同一個交易提交**：中斷時⛔ 不留下半份結果。
4. 順帶修同一類的競爭：backend 的取消只在任務仍是 `pending` 時生效（同樣用條件式 `UPDATE`；使用者 2026-11-18 同意納入，條件見 5.2）。
5. `execution_owner` 是內部欄位：python-server 的 `GET /backtest/{job_id}`（現在是 `SELECT *`）與 backend 的查詢都⛔ 不能把它帶到回應裡（v1 review P2）。

**不做**

* 取消執行中的任務、長時間任務的逾時或心跳。
* 多副本、`--reload`、uvicorn 多 workers 下的正確性（D2 的前提；見第三節）。
* 前端畫面；API 回應的欄位⛔ 不增減（`execution_owner` ⛔ 不對外輸出，目標 5）。
* 修改 live 的 `deploy.sh`（D4，另列 T-084）。
* 已經存在的重複交易明細：第七節第 0 步只查、⛔ 不清；查到再另外處理。
* 修正舊資料的時間：舊的 `started_at`／`finished_at` 有沒有偏移、偏多少，取決於當時連線的時區，⛔ 不能一律套用到全部舊資料；只記錄查證結果。
* 自動處理舊資料（`execution_owner` 是 NULL 的 `running`）：第六節的首次上版流程，由使用者確認清單後手動處理。

## 二、需求裁決（2026-11-18）

| # | 裁決 |
|---|---|
| D1 | 遺留的 `running` 改成 `failed`，寫入 `finished_at` 與明確原因 `interrupted_on_restart`。⛔ 不自動重跑；使用者重新送出時建立新任務（避免 OOM 的重跑循環）。 |
| D2 | 本次只加 `execution_owner`，值是 `worker` 或 `server`。**前提要寫明**：同一個 DB，每種服務只有一個執行程序，新舊程序⛔ 不能重疊。單一 container ⛔ 不代表單一程序；之後若支援多副本、reload 或多 workers，就必須改用實例識別與存活判定。 |
| D3 | 兩條路徑都保留：HTTP 提供即時觸發，worker 補撿還沒被認領的 `pending`。只記下服務名稱⛔ 不能防重複，兩邊都必須用同一套原子認領。HTTP 那邊認領之後，即使回應遺失，worker 也⛔ 不能接手重跑。 |
| D4 | 本次⛔ 不改 live 的 `deploy.sh`。部署規範保留回測的 `pending`／`running` 雙零檢查，部署期間⛔ 不新增回測；自動檢查另列 todo（T-084）。 |

另外兩點（裁決時一併要求）：

* 舊資料裡 `execution_owner` 是 NULL 的 `running`，⛔ 不能猜是哪個服務留下的；第一次上版時，在兩個舊的 python 程序都停掉之後，由使用者確認並處理（第六節）。
* 結果、交易明細與 `done` 在同一個交易提交；驗收要涵蓋兩條路徑競爭、兩個服務各自重啟，以及 HTTP 回應遺失（第七節）。

## 三、前提：每種服務一個執行程序

收掉遺留 `running` 的依據是「我是這種服務唯一的程序，所以所有 `execution_owner = 我` 的 `running` 都是上一個程序留下的」。這個前提目前成立的方式，以及會讓它不成立的情況：

| 項目 | 現況 | 會破壞前提的變更 |
|---|---|---|
| python-worker | compose 一個 service、`python worker.py` 單一程序、依序執行 | `--scale`、多副本、在 container 裡另開執行緒或程序跑任務 |
| python-server | compose 一個 service、`python http_server.py`（`uvicorn.run`，沒有 `workers`、沒有 `reload`）；回測在 FastAPI 的背景工作裡執行，與 HTTP 同一個程序 | uvicorn `workers>1`、`--reload`、gunicorn 多 worker、多副本 |
| 新舊重疊 | compose 的重建是先停舊的再起新的；`deploy.sh` 是 `down` → `up` | 「先起新的再停舊的」的滾動更新 |
| 本機的 `python/start_server.sh` | 用 `uvicorn … --reload`，只給本機開發 | ⚠️ 不在這個保證範圍內：reload 會換程序，舊程序的背景工作被中斷，新程序啟動時收掉它們剛好是對的；但若檔案變更時舊程序還沒停、新程序先起，就會誤收。⛔ 不用它連 dev 或 live 的 DB |

這張表要歸檔到主題文件（第九節），並在程式的收尾函式旁用註解指向它。

## 四、資料 contract 的變化

### 4.1 migration `077_backtest_execution_owner`（三種 DB 同步）

* `backtest_jobs` 新增 `execution_owner`：PostgreSQL／MySQL 是 `VARCHAR(16)`，SQLite 是 `TEXT`；可為 NULL、沒有預設值。值只會是 `worker`、`server` 或 NULL（還沒被認領，或上版前的舊資料）。
* ⛔ 不加 CHECK constraint（三種 DB 的語法與既有慣例不一）；值由程式控制、由測試鎖住。
* Down：`DROP COLUMN`（SQLite 比照 031 的寫法）。
* 屬於「加可為 NULL 的欄位」：照 `development-workflow.md`「回滾：live 沒有執行 goose Down 的入口」，回滾程式時⛔ 不需要退 schema——舊的 Go 與 python 都不讀這一欄。
* Go 的查詢都列明欄位，新增欄位⛔ 不影響 `GetJob`／`ListJobs`。

### 4.2 狀態轉移（新）

| 轉移 | 誰 | SQL 的條件 | 影響 0 列時 |
|---|---|---|---|
| 建立 → `pending` | backend | （INSERT） | — |
| `pending` → `running` | worker 或 server | `WHERE job_id = :id AND status = 'pending'`，同時寫 `execution_owner = :me`、`started_at = CURRENT_TIMESTAMP` | 認領失敗：⛔ 不執行、⛔ 不寫任何東西 |
| `pending` → `failed`（取消） | backend | `WHERE job_id = :id AND status = 'pending'` | 回 409（已經被認領了） |
| `running` → `done` | 認領者 | `WHERE job_id = :id AND status = 'running' AND execution_owner = :me`；跟結果、交易明細同一個交易 | 整個交易 rollback（結果與明細都⛔ 不留） |
| `running` → `failed`（執行出錯） | 認領者 | 同上的條件 | 記 log，⛔ 不再寫 |
| `running` → `failed`（`interrupted_on_restart`） | 同一種服務的下一個程序，啟動時 | `WHERE status = 'running' AND execution_owner = :me` | — |

* 時間一律用 DB 的 `CURRENT_TIMESTAMP`（跟 Go 一致；使用者 2026-11-18 同意）：`started_at` 寫在認領的 `UPDATE` 裡，`finished_at` 寫在 `done`／`failed`（含取消與 `interrupted_on_restart`）的 `UPDATE` 裡，⛔ 不再由 python 組時間字串。⚠️ 現在 python 寫的是 `datetime.utcnow().isoformat()`，是不帶時區的字串，寫進 PostgreSQL 的 `TIMESTAMPTZ` 時會照連線的 TimeZone 解讀；連線的 TimeZone 不是 UTC 的話，舊的 `started_at`／`finished_at` 會差一個時區。第七節第 0 步用 live 的資料與連線的 TimeZone 查證；⚠️ 偏移與否要看**寫入當時**的連線時區，⛔ 不能用現在的設定直接推論全部舊資料。不論結果都⛔ 不改舊資料，只記下來。
* `error` 欄的值：取消維持 `cancelled by user`；重啟收掉的是 `interrupted_on_restart`；執行出錯維持例外訊息。

## 五、程式的修改

### 5.1 python

* **`backtest/db_writer.py`**：拿掉無條件的 `update_job_status`，改成：
  * `claim_job(job_id, owner) -> bool`：上表的認領 `UPDATE`，回傳是否影響 1 列。
  * `finish_job(job_id, owner, result, trades) -> bool`：一個交易裡寫結果（現有的 upsert）、交易明細、條件式的 `done`；`done` 影響 0 列就 rollback 並回傳 False。
  * `fail_job(job_id, owner, error) -> bool`：條件式的 `failed`。
  * `reap_interrupted(owner) -> int`：啟動時收掉自己的遺留 `running`，回傳筆數並記 log（筆數與 job_id）。
  * `owner` 只接受 `worker`、`server`，其他值直接報錯。
* **`worker.py`**：啟動時先等 schema（5.3）、再 `reap_interrupted("worker")`，然後才開始輪詢。輪詢仍撈 `pending`，但每一筆先 `claim_job`；認領失敗就記 log、跳過。
* **`http_server.py`**：
  * lifespan：連線檢查之後等 schema、`reap_interrupted("server")`。
  * `POST /backtest`：**在回應之前同步認領**（用 threadpool 執行 DB 呼叫，⛔ 不卡住 event loop）。認領成功 → 排背景工作、回 202（回應內容不變）；認領失敗 → 回 409、⛔ 不排背景工作。背景工作只負責執行與 `finish_job`／`fail_job`。
  * 這樣 HTTP 回應遺失時，任務已經是 `running`／`server`，worker 撈不到；server 程序如果在背景工作開始前就掛了，下一次啟動會收掉它。
  * `GET /backtest/{job_id}`：改成**明列欄位**的 `SELECT`（就是現在對外的那些欄位，跟 Go 的 `GetJob` 同一組），`execution_owner` ⛔ 不查；`_backtest_job_payload` 的 `trigger_source` → `trigger` 改名照舊。不用「`SELECT *` 再刪掉」的做法：之後再加內部欄位又會漏出去。
* 現有的測試要改與新增：
  * `test_http_server_backtest_submit.py`：202 前已經認領；新增 409 的情況。
  * `test_http_server_backtest_get.py`：**回應契約測試**——`job` 的鍵集合要剛好等於對外欄位的集合（含 `trigger`、不含 `trigger_source`、不含 `execution_owner`）；DB 列多出欄位時也⛔ 不能出現在回應裡。

### 5.2 Go

* **取消**：新增 repo 方法，用 `WHERE job_id = ? AND status = 'pending'` 改成 `failed`（`error = 'cancelled by user'`、`finished_at = CURRENT_TIMESTAMP`），回傳影響列數。**只有影響 1 列才算取消成功**（200）。影響 0 列時再查一次這筆：不存在 → 404；存在（已被認領或已結束）→ 409，訊息跟現在「只能取消 pending」一致；任何 DB 錯誤 → 500（`serverError`）。⚠️ 現在的 handler 把 `GetJob` 的所有錯誤都當成 404，要改成只有「查無此列」才是 404。現在是先讀狀態、再無條件更新，中間被認領的話會把執行中的任務標成 `failed`，之後 `done` 又蓋回去。
* **觸發 HTTP 的 log**：現在不管狀態碼都記 `python http trigger ok`，連線失敗時記「job will be picked up by worker」。改成：2xx 記 ok；409 記「已被認領」；其他狀態碼記 warn；連線失敗或逾時的訊息改成「結果未知：python-server 可能已經認領，⛔ 不能當成未認領；worker 只會接手仍是 pending 的任務」。不論哪一種都⛔ 不改任務狀態。
* Go 的 `GetJob`／`ListJobs` 已經是明列欄位、`store.BacktestJob` 沒有這個欄位，所以 backend 的回應⛔ 不會帶出 `execution_owner`；用既有的 handler 測試確認回應的鍵集合不變。
* 既有的 `UpdateJobStatus` 的 `running` 分支沒有呼叫者；保留或刪除在實作時確認（刪除就同步改測試）。

### 5.3 上版時的 schema 時序

compose 裡 python 兩個服務⛔ 不等 backend；migration 是 backend 啟動時才套用。`deploy.sh` 的 `up` 會同時起三個服務，新的 python 可能在 077 套用之前就啟動，認領的 SQL 會因為沒有這一欄而失敗。

做法：python 兩個服務啟動時先確認 `backtest_jobs.execution_owner` 存在，不存在就每 2 秒再查，最多 120 秒；逾時就以非 0 結束（交給 compose 重啟，log 寫明原因）。正常上版時 backend 在幾秒內就套用完，python 的 `RestartCount` 應該維持 0。⛔ 不在 python 端建立欄位（schema 只由 goose 管）。

## 六、部署與舊資料

### 6.1 之後的一般部署

照 `development-workflow.md`「部署腳本：失敗即停」，開始條件含回測的雙零檢查（`pending`／`running` 都是 0），部署期間⛔ 不新增回測（D4）。

### 6.2 首次上版（含 077）的流程

v1 的寫法有順序矛盾（v1 review P2）：開始條件要求 `running` 是 0，卻把舊的 `running` 安排在部署後處理；如果上版前就有卡住的任務（舊程式的 I-134 本身就會造成），開始條件永遠不成立。首次上版改成下面的流程，**只在這一次**把「`running` 是 0」換成「每一筆 `running` 都已由使用者確認是遺留的」：

1. **停止新提交**：從第 2 步起到第 6 步做完，操作者⛔ 不從前端或 API 送出回測。
2. **部署前的唯讀盤點**：列出 `backtest_jobs` 的 `pending` 與 `running`（job_id、`started_at`、`created_at`）。
   * `pending` 必須是 0（否則 build 期間舊的 worker 會接走、`down` 時又變成遺留）。不是 0 就等它們跑完或由使用者決定，⛔ 不開始。
   * 每一筆 `running` 由使用者判斷：還在執行（`started_at` 很近，python 的 log 還在處理這個 job）→ 等它結束再重新盤點；已經沒有程序在執行（遺留）→ 列入**待處理清單**。清單確定後才進下一步。
3. **記下舊程序**：兩個 python container 的 container ID 與 `StartedAt`。
4. **部署**（`deploy.sh`：`down` 停掉兩個舊程序 → `up` 起新程序）。新程式的收尾只動 `execution_owner = 自己` 的列，⛔ 不會碰 NULL 的舊資料。
5. **確認兩個舊程序都已退出**：第 3 步記下的 container ID 都已經不在（`docker ps -a` 查不到，或 `State` 不是 running），新的 container ID 不同。確認之前⛔ 不處理任何一筆。
6. **處理清單**：再查一次 `status = 'running' AND execution_owner IS NULL` 的列。
   * 只處理「在待處理清單裡、而且仍是 NULL 的 `running`」：改成 `failed`、`error = 'interrupted_on_restart'`（或使用者指定的原因）、`finished_at = CURRENT_TIMESTAMP`；SQL 的條件同時帶 job_id 清單、`status = 'running'`、`execution_owner IS NULL`，在交易裡先比對影響列數等於清單筆數，不符就 rollback。SQL 由 Claude 提供、使用者執行。
   * 查到清單以外的 NULL `running`（第 2 步到 `down` 之間舊程序又開始了一筆）：舊程序已在第 5 步確認退出，所以它也是遺留；列給使用者確認後再照同樣的方式處理，⛔ 不自動併入。
7. 之後的部署回到 6.1。
* **回滾**：python 與 backend 退回 `pre-<案號>` 的 image；schema ⛔ 不用退（4.1）。退回後舊程式又變成無條件更新，新欄位只是不再被寫入。

## 七、測試與驗證

0. **live 的唯讀查詢（寫計畫期間，使用者執行）**：
   * `backtest_jobs` 各狀態的筆數，`running` 的 job_id 與 `started_at`。
   * 有沒有重複的交易明細：同一個 `job_id` 下，`symbol`、`direction`、`entry_time`、`exit_time` 都相同的列出現兩次以上。有的話列出 job_id 與筆數，⛔ 不清。
   * 時區：比較 python 寫入的 `started_at` 與 Go 寫入的 `created_at` 的差值（正常應該是秒級；差 8 小時左右就是 4.2 的時區問題）。
1. **python 單元測試**（`python/scripts/test.sh`；用 SQLite 檔案 DB，從 repo 的 SQLite migration 建表）：
   * 認領：`pending` 才成功；非 `pending` 回 False 且⛔ 沒有任何寫入；兩個連線（兩個執行緒）同時認領同一筆，只有一個成功。
   * `finish_job`：結果、明細與 `done` 一起提交；在寫明細時注入失敗 → 結果與明細都沒有、狀態仍是 `running`；任務已不是自己的 `running` → rollback、回傳 False。
   * 收尾：只動 `execution_owner = 自己` 的 `running`；另一種服務的與 NULL 的⛔ 不動；寫入 `failed`、`interrupted_on_restart`、`finished_at`。
   * worker：認領失敗的那一筆⛔ 不執行。
   * HTTP：認領成功才排背景工作、回 202；認領失敗回 409、⛔ 沒有排背景工作；回應之前任務已經是 `running`／`server`。
   * schema 等待：欄位不存在時重試、逾時以非 0 結束；存在時立即通過。
   * `owner` 的值檢查。
2. **Go 測試**（`backend/scripts/test.sh`，SQLite）：取消 `pending` → 200、寫入 `failed`／`cancelled by user`／`finished_at`；已被認領或已結束 → 409、狀態⛔ 不變；不存在 → 404；DB 錯誤（查詢或更新失敗）→ 500、⛔ 不是 404；認領與取消同時發生時只有一方成功。觸發 HTTP：2xx、409、其他狀態碼、連線失敗、逾時各記對應的 log，任務狀態都⛔ 不變。回應的鍵集合⛔ 不含 `execution_owner`。
3. **migration**：Go 的 SQLite migration 測試、`scripts/test-postgres-migrations.sh`、`scripts/test-mysql-migrations.sh`（Up／Down 都要過）。
4. **dev 實測**（新增 repo 腳本，見下；只用 `docker-compose.dev.yml` 的 dev project）：
   * **兩條路徑競爭**（誰贏都合法，驗的是「只執行一次」）：backend 送出多筆回測（HTTP 與 worker 都開著），每一筆的 `execution_owner` 只有一個、交易明細沒有重複、server 與 worker 的 log 合起來每一筆只執行一次。為了讓競爭真的發生，worker 的輪詢間隔設成最短，並另外直接把任務寫成 `pending` 後同時打 `POST /backtest`。
   * **python-server 單獨重啟**：server 執行中的任務，砍掉 server 再起來 → 該筆變成 `failed`／`interrupted_on_restart`；worker ⛔ 沒有接手、worker 自己的任務⛔ 不受影響。
   * **python-worker 單獨重啟**：同上，角色對調（先停 server，讓 worker 認領）。
   * **HTTP 回應遺失**（v1 review P2：「很短的逾時後斷線」⛔ 不能保證 server 已經認領，worker 合法搶先認領也會讓測試失敗，所以要控制時序）：
     1. 先暫停 worker（`docker compose pause python-worker`），讓這一筆只有 server 能認領。
     2. 直接寫入一筆測試用的 `pending`，對 python-server 送 `POST /backtest`；**等到 DB 看到這一筆已經是 `running`／`server`（認領已提交）**，才把 client 端的連線關掉、⛔ 不讀回應（等同回應遺失；server 那邊照常執行背景工作）。認領在回應之前同步完成（5.1），所以這個時間點一定在回應送出之前或同時。
     3. 恢復 worker，至少等 3 輪輪詢。
     4. 驗證：worker 的 log 沒有處理這個 job_id；這一筆最後由 server 完成成 `done`、`execution_owner` 仍是 `server`；交易明細只有一份。
     * 單元測試另外鎖住同一個不變式：認領提交之後、回應之前就讓 handler 失敗或取消，任務仍是 `running`／`server`，worker 的認領回 False。
   * **schema 時序**：用沒有 077 的 DB 先起 python，再讓 backend 套用 → python 在等待之後正常啟動。
   * 腳本：`scripts/smoke-dev-backtest-claim.sh`，加上假 docker 的測試 `scripts/test-smoke-dev-backtest-claim.sh`（掛進 `python/scripts/test.sh`）。使用者 2026-11-18 同意納入；⚠️ **實作前先把腳本細節列給使用者確認**，至少包含：
     * **dev project 限制**：比照 `smoke-dev-worker.sh`——compose 檔固定、改變狀態之前驗證 project 是 `stock_trading_dev`、擋 `COMPOSE_PROJECT_NAME`。
     * **測試資料隔離**：測試任務的 job_id 用固定前綴（例如 `i134smoke_<時間>_`），驗證與清理都只針對這個前綴；開始前 dev DB 有任何非本次的 `pending`／`running` 就停下（否則會被一起執行或誤判）；結束時刪除本次的任務、結果與明細，⛔ 不動其他列。
     * **每個 docker 呼叫的上限**：`timeout -k`（同 `smoke-dev-worker.sh` 的 `KILL_AFTER`），等待迴圈受總時限約束。
     * **失敗收尾**：成功、失敗、逾時、Ctrl+C 都要恢復被暫停的 worker、清掉本次的測試資料，並把 python 兩個服務恢復到開始前的狀態；收尾失敗回傳非 0。
     * **同步方式**：上面「HTTP 回應遺失」的做法——以 DB 看到認領已提交為準，再切斷回應；⛔ 不靠固定的 sleep 或逾時。
5. **突變**：拿掉認領的 `AND status = 'pending'`、把 `finish_job` 拆成兩個交易、拿掉收尾的 owner 條件，各自要讓對應的測試失敗。

## 八、主要風險與因應

| 風險 | 因應 |
|---|---|
| 前提（每種服務一個程序）被之後的變更破壞，收尾誤殺別的程序正在跑的任務 | 第三節的表歸檔到主題文件，程式註解指向它；`http_server.py` 啟動參數若加 `workers`／`reload`，同一處的註解要求先改用實例識別 |
| 新 python 早於 migration 啟動 | 5.3 的等待（有上限、逾時非 0） |
| HTTP 改成同步認領後，`POST /backtest` 多一次 DB 往返 | 一次 `UPDATE`，毫秒級；backend 的 HTTP 逾時是 30 秒 |
| 409 讓 backend 誤判 | backend 只記 log、⛔ 不改狀態（5.2） |
| 長時間的交易（結果與明細一起提交） | 明細筆數就是交易次數，一次回測最多數百筆；⛔ 不需要分批 |
| 舊資料的 NULL `running` | 第六節由使用者確認 |

## 九、完成後要歸檔的位置

* `docs/architecture.md` 的 python 服務／回測那一節：兩條路徑、原子認領、狀態轉移表、收尾與第三節的前提。
* `docs/database-schema.md` 的 `backtest_jobs`：`execution_owner` 與狀態轉移的條件。
* `docs/development-workflow.md`：部署前的回測雙零檢查與「部署期間不新增回測」（D4）；`smoke-dev-backtest-claim.sh` 的用法。
* `docs/api-reference.md`（若有回測端點的段落）：python-server `POST /backtest` 的 409、backend 取消的 409 條件。

## 版本紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-18 | 初版：依 D1～D4 裁決與兩點附加要求。 |
| v1.1 | 2026-11-18 | review [P2]×3：首次上版另列流程（6.2：停止新提交、盤點並確認待處理清單、確認舊程序退出之後才處理）；python `GET /backtest/{job_id}` 改成明列欄位、加回應契約測試；HTTP 回應遺失的驗收改成以 DB 看到認領提交為準、先暫停 worker。範圍追加（使用者同意）：取消改條件式（200／404／409／500 的條件）、HTTP 觸發依狀態碼記 log 且逾時視為結果未知、時間統一寫在狀態轉移的 SQL、dev 驗收腳本的確認項目。 |
