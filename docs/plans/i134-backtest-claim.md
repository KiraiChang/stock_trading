# I-134：回測任務的原子認領與重啟後的遺留 `running`（計畫書 v1.3）

> 狀態：v1.1 已確認（使用者 2026-11-18）；v1.2（dev 驗收腳本的細節）已確認（2026-11-18）；v1.3（實作時發現的既有問題併入本計畫修正：Go 讀不到 `error` 為 NULL 的任務、worker 在 PostgreSQL 上讀到 JSONB 的 `symbols` 與 DATE 的日期都是原生型別、回測因此失敗；使用者 2026-11-18 同意）；**repo 與 dev 已實作（2026-11-18），待 review；live 的首次上版（6.2）還沒做**（實作備註在「八之後」）；實作原本暫緩到 T-083 部署到 live 完成之後（使用者 2026-11-18 指示；live 的 `deploy.sh` 會 `git pull`，T-083 部署前 push 的實作會被一起帶上 live）；**T-083 已於 2026-11-18 14:24 部署完成，可以開始實作**。dev 驗收腳本的細節：v1.2 依使用者 2026-11-18 的裁決改寫（第七節第 4 點）。對應：`docs/issue.md` I-134。需求裁決：2026-11-18（D1～D4，見第二節）。
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
* **認領的 log**（v1.2；dev 驗收用它判斷「兩條路徑都嘗試過同一筆」）：worker 與 server 的每一次認領，成功與失敗都記一行，帶 `job_id`、`owner` 與結果（例如 `claim ok job=… owner=worker`、`claim lost job=… owner=server`）。server 的 409 也算一次失敗的嘗試。
* **v1.3：worker 解析 `symbols`**（dev 實跑發現的既有 bug，使用者 2026-11-18 同意併入）：`backtest_jobs.symbols` 在 PostgreSQL 是 `JSONB`，psycopg2 讀出來已經是 list，`worker.py` 卻一律 `json.loads`，所以 PostgreSQL 上 worker 接到的每一筆都 `failed`（`the JSON object must be str, bytes or bytearray, not list`）；python-server 收到的是請求的字串，⛔ 不受影響。第七節第 0 步看到 live 的 8 筆都在建立後 21 毫秒內開始，跟「worker 這條路徑在 live 上沒有成功過」一致（⛔ 沒有直接查證）。修法：只有字串才 `json.loads`，list 直接用；list 與字串各一個單元測試。同一類的第二個（修完 `symbols` 重跑時發現）：`start_date`／`end_date` 是 DATE，psycopg2 讀出來是 `date` 物件，modular 回測用它切帶時區的索引會丟 `Cannot compare tz-naive and tz-aware datetime-like objects` → worker 一律轉成 `YYYY-MM-DD` 字串再交給回測（字串與 date 各一個單元測試）。
* 現有的測試要改與新增：
  * `test_http_server_backtest_submit.py`：202 前已經認領；新增 409 的情況。
  * `test_http_server_backtest_get.py`：**回應契約測試**——`job` 的鍵集合要剛好等於對外欄位的集合（含 `trigger`、不含 `trigger_source`、不含 `execution_owner`）；DB 列多出欄位時也⛔ 不能出現在回應裡。

### 5.2 Go

* **取消**：新增 repo 方法，用 `WHERE job_id = ? AND status = 'pending'` 改成 `failed`（`error = 'cancelled by user'`、`finished_at = CURRENT_TIMESTAMP`），回傳影響列數。**只有影響 1 列才算取消成功**（200）。影響 0 列時再查一次這筆：不存在 → 404；存在（已被認領或已結束）→ 409，訊息跟現在「只能取消 pending」一致；任何 DB 錯誤 → 500（`serverError`）。⚠️ 現在的 handler 把 `GetJob` 的所有錯誤都當成 404，要改成只有「查無此列」才是 404。現在是先讀狀態、再無條件更新，中間被認領的話會把執行中的任務標成 `failed`，之後 `done` 又蓋回去。
* **觸發 HTTP 的 log**：現在不管狀態碼都記 `python http trigger ok`，連線失敗時記「job will be picked up by worker」。改成：2xx 記 ok；409 記「已被認領」；其他狀態碼記 warn；連線失敗或逾時的訊息改成「結果未知：python-server 可能已經認領，⛔ 不能當成未認領；worker 只會接手仍是 pending 的任務」。不論哪一種都⛔ 不改任務狀態。
* Go 的 `GetJob`／`ListJobs` 已經是明列欄位、`store.BacktestJob` 沒有這個欄位，所以 backend 的回應⛔ 不會帶出 `execution_owner`；用既有的 handler 測試確認回應的鍵集合不變。
* 既有的 `UpdateJobStatus` 的 `running` 分支沒有呼叫者；保留或刪除在實作時確認（刪除就同步改測試）。→ 實作：取消改用新的 `CancelPendingJob` 之後 `UpdateJobStatus` 沒有任何呼叫者，已刪除（沒有既有測試）。
* **v1.3：`GetJob`／`ListJobs` 讀不到 `error` 為 NULL 的任務**（實作時發現的既有 bug，使用者 2026-11-18 同意併入本計畫）。backend 建立任務時不寫 `error`、python 認領時也不寫，所以 `pending`／`running` 的 `error` 是 NULL；`store.BacktestJob.Error` 是 `string`，sqlx 掃描時失敗（`converting NULL to string is unsupported`）。影響（I-134 之前就存在）：`GET /backtest/:id` 對 `pending`／`running` 回 404；`GET /backtest` 只要有一筆 `pending`／`running` 就回 500；`DELETE` 取消 `pending` 也回 404，取消從來沒有成功過。條件式取消影響 0 列時要靠 `GetJob` 分辨 404／409，不修的話 `running` 會變成 500。修法：兩個查詢改成 `COALESCE(error, '') AS error`；對外是 `error,omitempty`，JSON 不變（NULL 與空字串都不輸出）。⛔ 不改型別、⛔ 不改既有資料。測試：`pending`／`running` 的 GET、列表、取消（用真的 SQLite repo）。

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
   * ✅ **結果（2026-11-18 10:xx，使用者在 live 執行）**：
     * `backtest_jobs` 共 8 筆：`done` 6、`failed` 2；`pending`／`running` 都是 0 → 目前⛔ 沒有遺留的任務，6.2 的待處理清單預期是空的（上版當天仍要重新盤點）。
     * 重複的交易明細：0 個 job → 兩條路徑的競爭在 live 上⛔ 沒有留下重複（資料量很少，⛔ 不代表競爭不存在）。
     * 目前連線的 TimeZone 是 `UTC`；這 8 筆（全部是 2026-07-01 建立的）的 `started_at` 與 `created_at` 相差 0.004～0.021 秒、負值 0 筆，⛔ 沒有觀察到約 8 小時的落差。⚠️ 歷史寫入時的連線時區⛔ 沒有直接確認（review 2026-11-18：原本寫「證明寫入時的連線是 UTC」超出查詢能證明的範圍）。
     * 8 筆的「開始減建立」都在 21 毫秒內，比 worker 的輪詢間隔（`python/config.yaml` 的 10 秒）短得多，8 筆都剛好被 worker 在 21 毫秒內輪詢到的機率很低，看起來是 HTTP 路徑認領的；但 DB 沒有記執行者，⛔ 無法確認（077 之後才有 `execution_owner`）。
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
4. **dev 實測**（新增 repo 腳本 `scripts/smoke-dev-backtest-claim.sh`；只用 `docker-compose.dev.yml` 的 dev project；v1.2 依使用者 2026-11-18 的裁決改寫）
   * **範圍**（裁決 D1）：腳本直接用 psql 寫入測試任務、直接呼叫 python-server 的 `POST /backtest`（`127.0.0.1:18001`），⛔ 不經過 backend。dev smoke 驗的是 **python 的認領與持久化**；backend 的條件式取消、HTTP 觸發與 log 由第 2 點的 Go 測試負責。⚠️ 這次**沒有驗完整的 backend 提交流程**（backend 收到請求 → INSERT → 觸發 HTTP → python 認領）。
   * **dev project 限制**：比照 `smoke-dev-worker.sh`——compose 檔固定是 `docker-compose.dev.yml`、⛔ 不接受覆寫；擋 `COMPOSE_PROJECT_NAME`（空字串也擋）；任何改變狀態的指令之前確認解析出來的 project 是 `stock_trading_dev`。
   * **開始前的檢查**（任何一項不符就 rc 2、⛔ 不改任何東西）：postgres 與 python-server 在跑；dev DB 已有 `execution_owner`（backend 已套用 077）；dev DB ⛔ 沒有任何 `pending`／`running`；worker 沒有處於 `paused`；測試標的在日期範圍內有日 K（標的、日期、策略都有預設值，可用環境變數換；策略在實作時挑一個最快、而且在預設範圍內**至少產生一筆交易**的）。
   * **開始時記錄服務狀態**：python-server 的 container ID；python-worker 是「不存在」「存在但停止」「在跑」三者之一，連同 container ID。收尾依這份紀錄恢復。
   * **image**：build python 兩個 image（上限見下），`up -d --no-deps python-server` 確保跑的是新 image；worker 的 image 只認 `stock_trading_dev-python-worker`。
   * **測試資料隔離**：job_id 一律是 `i134smoke_<執行時間>_<情境>_<序號>`；前綴只允許 `[a-z0-9_]`，由腳本驗證。驗證與清理的 SQL 用**字面比對**（`starts_with(job_id, :'prefix')`，前綴用 psql 變數傳入），⛔ 不用 `LIKE`（`_` 會被當成萬用字元）。
   * **上限**：每個 docker／psql 呼叫都用 `timeout -k "$KILL_AFTER"`（預設 `CALL_TIMEOUT=30`、`KILL_AFTER=5`）；整體上限 `TOTAL_TIMEOUT`；**build、持鎖連線**與每個等待迴圈的上限都取「自己的預設值」與「總剩餘時間」的較小值；收尾另有 `CLEANUP_TIMEOUT`（逾時之後仍要能收尾，所以⛔ 不受總剩餘時間限制）。
   * **持鎖的做法**（裁決 D2，重啟兩個情境共用）：
     * 背景開一個 psql（`PGAPPNAME=i134smoke_<執行時間>_lock`），在交易裡 `LOCK TABLE backtest_trades IN SHARE MODE`，再 `pg_sleep(上限)`；記下它的 PID。鎖的是**交易明細表**：`finish_job` 先寫結果、再寫明細，所以被擋住時結果已經寫進同一個交易、還沒提交，程序被砍後要驗證**結果也一起回滾**。（鎖 `backtest_results` 會擋在結果寫入之前，就算結果與明細被錯拆成兩個交易，測試也照樣通過。）
     * **同步點**：被測服務的 DB 連線（用 container 在 dev 網路上的 IP 比對 `pg_stat_activity.client_addr`）的 `pg_blocking_pids()` 包含本輪持鎖的 PID。⛔ 只看 `wait_event_type='Lock'` 不算。同時確認這個連線在 `backtest_results` 上已持有鎖（`pg_locks`），作為「結果已在同一個交易裡寫入」的證據。
     * 測試任務若沒有產生交易，`finish_job` 不會碰到明細表、任務會直接完成、等不到同步點 → 這個情境判**失敗**（⛔ 不算通過）。
     * 放鎖：`pg_terminate_backend(持鎖 PID)`，再結束背景的 psql。
   * **情境**（預設全部跑，可用 `SCENARIOS=` 挑選）：
     * **`race`**（裁決 D3）：輪詢間隔維持 `config.yaml` 的值，⛔ 不為測試改產品設定。在一個交易裡寫入 5 筆 `pending`（worker 每輪最多撈 5 筆），等到 worker 的 log（從寫入前的時間點起算）出現撈到 5 筆的那一輪，才並行送出這 5 筆的 `POST /backtest`。⛔ 不把「pause 超過輪詢間隔就一定會競爭」當成保證；**通過條件以證據為準**：
       * 至少有一筆任務，worker 與 server 的認領 log 都有出現（一方 `claim ok`、另一方 `claim lost` 或 409）——也就是兩條路徑確實都嘗試認領過同一筆；**沒有這個證據就判失敗**（未覆蓋競爭），⛔ 不算通過。
       * 每一筆：最後是 `done`、只有一個 `execution_owner`，且等於 `claim ok` 那一方；兩邊的 `claim ok` 合計剛好 1 次；結果 1 列；交易明細沒有重複。
       * 兩邊各贏幾筆只做記錄，⛔ 不要求兩邊都贏過。
     * **`server-restart`**：持鎖 → worker 認領 W（只寫入、不打 HTTP）並卡在明細 → 暫停 worker → 寫入 S 並 `POST`，server 認領並卡在明細 → 對 server `docker compose kill` → 起回 server → S 是 `failed`／`interrupted_on_restart`、有 `finished_at`、結果與明細都 0 列；W 仍是 `running`／`worker` → 放鎖、恢復 worker → W 是 `done`。
     * **`worker-restart`**：角色對調（S 先被 server 認領並卡住，再讓 worker 認領 W 並卡住，砍 worker、起回）→ W `failed`／`interrupted_on_restart`、結果與明細 0 列；S 仍 `running`／`server` → 放鎖 → S `done`。
     * **`lost-response`**（v1.2 review：也用明細表的鎖，免得快速的回測在第一次查詢前就變成 `done`，讓正確的實作也驗收失敗）：
       1. 先暫停 worker，讓這一筆只有 server 能認領；開始持鎖。
       2. 寫入一筆 `pending`；用**確定不讀取回應的 socket client**（腳本內附的小 python 程式，用 `python3 -I` 在主機上執行：連線、送出完整的請求、印出 `sent`，之後⛔ 不呼叫任何讀取，只等著被結束）送出 `POST /backtest`。⛔ 不用 `curl`：DB 看到認領的時候，`curl` 可能早就讀到 202 並結束。
       3. 等到**同步點**：這一筆是 `running`／`server`（認領已提交），而且 server 的連線被本輪持鎖的 PID 擋住（同重啟情境的判斷）。這時任務一定還沒完成。接著結束 client（`SO_LINGER` 0，關閉時直接 RST，丟掉 server 送來、尚未讀取的回應）。
       4. 恢復 worker，等到 worker 的 log 出現恢復之後的至少 3 輪輪詢；這段期間這一筆仍是 `running`／`server`，worker 的 log 沒有這個 job_id。
       5. 確認 worker 沒有接手之後才放鎖，等這一筆完成。
       6. 驗證：最後是 `done`、`execution_owner` 仍是 `server`；worker 的 log 從頭到尾沒有這個 job_id；結果 1 列、交易明細只有一份（沒有重複，筆數等於結果的 `total_trades`）。
     * **`schema`**（裁決 D4）：只驗「**欄位出現後，python 能繼續啟動**」；077 的正確性由第 3 點的 migration 測試負責。
       1. 在 dev postgres 建臨時 DB `i134smoke_<執行時間>`，用 `pg_dump -s` 複製回測三張表的結構，再拿掉 `execution_owner`。
       2. 用 `compose run --rm --no-deps` 起一個臨時的 python-server（`DATABASE_DSN` 指向臨時 DB、**⛔ 不發布 port**、名稱帶本輪前綴），記下 container ID；確認 log 顯示它在等 schema。
       3. 手動 `ALTER TABLE … ADD COLUMN execution_owner VARCHAR(16)` → 確認它接著完成啟動（log）。
       4. 先停掉這個 container（依記下的 ID）、`pg_terminate_backend` 結束連到臨時 DB 的所有連線，再 `DROP DATABASE`。
       * 臨時 server 只連臨時 DB，它的收尾（`reap_interrupted`）也只作用在臨時 DB，⛔ 不違反第三節的前提。
   * **收尾**（trap EXIT／INT／TERM；成功、失敗、逾時、Ctrl+C 都走同一段；依序）：
     1. **讓兩條執行路徑停止處理本次的任務**：本次前綴下仍是 `pending` 的改成 `failed`（`error` 記 `i134smoke_cleanup`），之後沒有路徑能再認領；接著停掉 python 兩個服務（暫停中的 worker 先 `unpause` 再立刻 `stop`；此時鎖還在，卡住的任務提交不了）。
     2. **確認執行路徑已停止**：python 兩個服務的 container 都不在執行（`State.Running` 為 false、⛔ 不是 `paused`），而且 `pg_stat_activity` 裡⛔ 沒有來自它們的連線（用開始時記下的 container IP 比對，持鎖連線除外）。
     3. 結束 socket client 與持鎖的連線——這兩項是獨立的資源，⛔ 不論第 2 步的結果都要做（盡力而為，失敗就記下）。
     4. **只有第 2 步確認通過才做**：依前綴刪除本次的交易明細、結果、任務。
     5. 臨時 DB：先停掉臨時 server（依記下的 ID）並確認它已不在執行、結束連到臨時 DB 的連線；**確認之後**才 `DROP DATABASE`。這一項只依賴臨時 server，跟第 2 步無關。
     6. **只有第 2 步確認通過才做，而且最後才做**：依開始時的紀錄恢復服務。python-server 起回來；worker 原本在跑就起回來，原本存在但停止就維持停止（⛔ 不移除），原本不存在才移除本次建立的 container。⚠️ 開始前已確認 dev 沒有其他 `pending`／`running`；執行期間若有人在 dev 另外送出回測、剛好被停掉，重啟後的收尾會把它標成 `interrupted_on_restart`（符合新的語意，dev 才會遇到）。
     * **第 2 步沒有確認通過**（停止失敗、逾時、查詢失敗）：⛔ 不刪測試資料、⛔ 不恢復服務（避免跟仍在執行的任務競爭）；印出本次前綴、各服務的狀態與手動清理的 SQL，回傳非 0。
     * 任何一步失敗，最後都回傳非 0；除了上面「依賴第 2 步」的兩步，其他步驟失敗時仍繼續往下做。
   * **假 docker 測試** `scripts/test-smoke-dev-backtest-claim.sh`（掛進 `python/scripts/test.sh`）：參數與 project 檢查；開始前的檢查擋下時⛔ 沒有改變狀態；清理的 SQL 只帶本次前綴、用字面比對；收尾的順序（停止路徑 → 確認停止 → 放鎖 → 刪資料 → 恢復服務）；停止失敗或確認不通過時⛔ 沒有刪資料、⛔ 沒有恢復服務，但 socket client 與持鎖連線仍被結束；臨時 server 沒有確認停止時⛔ 不刪臨時 DB；worker 三種開始狀態各自恢復正確（存在但停止的⛔ 不被移除）；失敗、逾時時暫停的 worker 一定被處理、持鎖連線被結束；卡住又忽略 SIGTERM 的呼叫被 `-k` 砍掉、⛔ 沒有孤兒；race 沒有雙方嘗試的證據時判失敗；收尾失敗回傳非 0。
   * **腳本的突變**：拿掉清理的前綴條件、`starts_with` 改成 `LIKE`、收尾改成先恢復服務再刪資料、把存在但停止的 worker 當成本次建立而移除、race 拿掉「雙方嘗試」的檢查、拿掉「確認停止才刪資料」的條件，各自要讓假 docker 測試失敗。
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

## 八之後：實作備註（2026-11-18；已改好、未 commit，待 review）

**改了什麼**

* migration `077_backtest_execution_owner`（PostgreSQL／MySQL `VARCHAR(16)`、SQLite `TEXT`；Down `DROP COLUMN`）。
* python：`backtest/db_writer.py` 改成 `claim_job`／`finish_job`／`fail_job`／`reap_interrupted`／`wait_for_schema`（拿掉 `update_job_status`、`write_result`、`write_trades`）；`worker.py` 每筆先認領、啟動時等 schema 再收尾；`http_server.py` 的 lifespan 等 schema 再收尾、`POST /backtest` 回應前同步認領（409）、`GET /backtest/{job_id}` 明列欄位且 payload 只留對外的鍵。v1.3 的兩個 worker 型別修正（`symbols`、日期）。
* Go：`CancelPendingJob`（條件式）取代 `UpdateJobStatus`；取消的 200／404／409／500；觸發 HTTP 依結果分開記 log；`GetJob`／`ListJobs` 的 `COALESCE(error, '')`。
* 測試：python `test_backtest_db_writer.py`（新）、`test_worker_claim.py`（新）、submit／get／startup 三支改寫；`python/scripts/test.sh` 把 backend 的 SQLite migration 唯讀掛進 container（`SQLITE_MIGRATIONS_DIR`），測試用它建表。Go `store/backtest_repo_test.go`、`api/handler/backtest_test.go`、`backtest/job_test.go`（都是新的；之前回測沒有任何 Go 測試）。
* 腳本：`scripts/smoke-dev-backtest-claim.sh`、`scripts/test-smoke-dev-backtest-claim.sh`（掛進 `python/scripts/test.sh`）。

**跟計畫的差異（除了 v1.3 已記的）**

* **收尾的「確認已停止」多了一步**：被鎖擋住的 backend ⛔ 不會自己發現 client 已經斷線（它在等鎖，不會去讀 socket），所以 container 停了之後，來自它的連線仍留在 `pg_stat_activity`、等鎖放開才會報錯結束。改成 container 確認不在執行之後，用 `pg_terminate_backend` 結束來自它們（開始時與每次重啟後記下的 IP）的連線，再確認為 0。情境裡砍掉服務之後也一樣，但只結束「來自那個 IP、被本輪鎖擋住」的 backend（新起來的 container 可能拿到同一個 IP）。
* dev 驗收的預設策略是 `breakout_swing_atr_v1`（計畫寫「實作時挑」）：`breakout_v1` 有交易就失敗（I-135，2026-11-19 已修正並收斂）。

**假 docker 測試抓到的腳本 bug（實跑時沒有暴露）**

* `KNOWN_IPS` 在 `$(…)` 裡更新、被 subshell 丟掉，收尾的「沒有來自它們的連線」實際上沒有檢查任何 IP → 改成 `remember_ip VAR CID`（`printf -v`）。
* socket client 經由 `t` 函式在背景執行，`$!` 是 subshell，kill 它會留下 `timeout` 與 `python3` → 改成直接呼叫 `timeout`；client 收到 TERM 時 RST 關閉後印 `closed`，腳本檢查這一行（之前實跑的 lost-response 很可能沒有真的 RST，修正後重跑通過）。

**驗證**

| 項目 | 結果 |
|---|---|
| python 相關測試（db_writer、worker、submit、get、startup） | 通過（含 v1.3 的 `symbols`／日期各 2 項） |
| Go：store、api/handler、backtest、database | 通過 |
| `scripts/test-postgres-migrations.sh`、`scripts/test-mysql-migrations.sh`（Up、分段 Down 到 0） | 都通過 |
| `scripts/test-smoke-dev-backtest-claim.sh` | 38 項通過 |
| 腳本突變（6 種：拿掉清理的前綴條件、`starts_with` 改 `LIKE`、先恢復服務再刪資料、移除原本停止的 worker、拿掉 race 的雙方證據、拿掉「確認停止才刪資料」） | 各自 1～5 項失敗 |
| python 突變（拿掉認領的 `AND status = 'pending'`、`finish_job` 拆成兩個交易、拿掉收尾的 owner 條件） | 6／6／2 項失敗 |
| dev 實跑 `scripts/smoke-dev-backtest-claim.sh`（修正後） | 五個情境全部通過：race 5 筆都有雙方認領紀錄（worker 贏 1、server 贏 4）；兩種重啟的回滾與不互相影響；回應遺失後 worker 三輪沒有接手；schema 等待後繼續。收尾後 dev 的 `backtest_*` 0 筆、沒有臨時 DB、沒有殘留連線與 container |
| dev 實跑 `NO_BUILD=1 scripts/smoke-dev-worker.sh` | 通過（worker 新的啟動流程） |
| shellcheck（兩支腳本） | 0 則 |
| `python/scripts/test.sh -q`（完整） | rc 0、135 秒：pytest 1963 passed、1 skipped；shell 測試（含 `test-smoke-dev-backtest-claim.sh` 38 項）都通過 |
| `backend/scripts/test.sh`（完整，SQLite） | rc 0 |

**review（2026-11-18）的 P2×3 與修正**

1. **臨時 server 可能留下、DB 卻被刪除**：建立 container 之後取 ID 失敗時，收尾跳過移除、仍然刪 DB（review 用假 docker 重現：server 仍 running、移除 0 次、`DROP DATABASE` 1 次）。→ 改成事先決定名稱、`run` 之前就標記「已起」，收尾一律依名稱 `rm -f`，再用名稱查（`docker ps -a --filter name=^/…$`）；查詢失敗或仍查得到就⛔ 不刪 DB。假 docker 新增三項（取不到 ID 又移不掉 → ⛔ 不刪；取不到 ID 但移得掉 → 才刪；查不到狀態 → ⛔ 不刪）；突變「改回用 ID」→ 3 項失敗。
2. **schema 等待沒有落實 120 秒上限**：探測的 DB 查詢沒有逾時，探測成功就直接回傳、⛔ 不看期限（review 模擬探測 300 秒、上限 120 秒仍回報成功）。→ 探測放在 daemon thread，只等剩餘時間（卡住就放棄、以 `SchemaNotReady` 結束；daemon 才不會擋住程序結束），探測回來之後再核對期限（過期算逾時），最後一次等待只睡到期限為止。新增三個單元測試（卡住的探測在上限內結束、過期才回來的成功算逾時、最後一次等待被截短）。
3. **收尾超時後仍持續取得新預算**：剩餘時間用完時固定給 1 秒，每個呼叫還能再加 `KILL_AFTER`（review：`CLEANUP_TIMEOUT=1`、`KILL_AFTER=1` 實際 10 秒）。→ 每個呼叫的上限＝截止時間的剩餘秒數扣掉 `KILL_AFTER`，用完就⛔ 不執行、印出「時限用完，未執行：…」並把收尾記成失敗；`timeout 0`（等於不設上限）由 `t`／`ti`／等待迴圈擋下。假 docker 新增「收尾開始之後每個 docker 呼叫都卡住又忽略 SIGTERM」：`CLEANUP_TIMEOUT=3` → 3 秒結束、列出沒做的項目、⛔ 沒有刪資料；突變「改回補 1 秒」→ 9 秒、測試失敗。

修正後：假 docker 測試 42 項通過；`python/scripts/test.sh -q` rc 0（pytest 1966 passed、1 skipped）；dev 實跑五個情境全部通過、收尾後沒有殘留。Go 這輪沒有改。

**live 首次上版（6.2；2026-11-18，使用者在 live 執行）**

* **時段**：17:18 開始、17:25 確認完成。⚠️ ⛔ 不在當初給的時段（14:05～14:50、15:12～15:50）內，而且是在 I-134 的 review 確認之前執行的（使用者決定）。開始時 `job_runs` 沒有 `running`；17:00 的 `sr_analysis`（id 5942）17:01:19 已經結束，距離超過 10 分鐘；下一個排程是 21:00。
* **第 1、2 步（停止新提交、盤點）**：17:18:31 `backtest_jobs` 的 `pending`／`running` 都是 0（`done` 6、`failed` 2）→ 待處理清單是空的。migration 版本 76；`deploy.sh` sha `abcf1c019b25`、`--preflight` 通過（MemAvailable 4919MiB）；三個服務 `running`、`RestartCount` 0（`2438fb0d1adf`、`bee594b80d5c`、`5520de9a372b`）；`Config.Env` 50 行、`a4eccbc0aa79`；backend 執行檔 `df45b9cffe0c`。
* **第 3 步（記下舊程序）**：python-server `3e50b66dd8f6`（StartedAt 06:20:21Z）、python-worker `cc3eb9fefae9`（06:20:20Z）；三個服務打上 `pre-i134`（ID 同上）。
* **第 4 步（部署）**：`git pull` `e61fbdf`→`7cca99a`；`config ok (production=true)`；`down` → `up -d --no-build`；`部署完成：backend /health 正常、RestartCount 0`，`deploy rc=0`。
* **第 5、6 步與部署後確認**（17:25:23）：
  * 三個服務 `running`、`RestartCount` 0，image 都換了（backend `c0e4e6f21e15`、python-server `aad99a5aac67`、python-worker `29b60134f355`）；新的 container 是 `d6dad08f386e`／`0a1cc344cf9b`，第 3 步的舊 ID 查不到（0）→ 舊程序已退出。
  * backend：`Config.Env` 50 行、`a4eccbc0aa79`（不變）；執行檔 `b07a2830c0fc`（Go 有改，預期不同）；`redis connected`／`scheduler started` 2 筆、設定錯誤 0 筆；`migrations applied` 的 `"version":77`，DB 的 goose 版本 77，`execution_owner` 欄位存在。
  * python：兩個服務都是 `schema ready`（server 09:24:59Z、worker 09:24:55Z；都⛔ 沒有等待）與 `no interrupted jobs to reap`（各自的 owner），`Traceback` 0；python-server `/health` `{"status":"ok"}`；worker 有 3 次 `poll #`。
  * 第 6 步：`running` 且 `execution_owner IS NULL` 的列 0 筆，⛔ 不需要處理；`pending`／`running` 仍是 0。
* 之後的部署回到 6.1。`pre-i134` 標籤保留到 review 確認後再清。
* ⚠️ worker 這條路徑在 live 上第一次能真的執行任務（v1.3 修了 JSONB／DATE 的型別）；上版後還沒有送過回測，第一筆回測的結果之後再看。

**還沒做**：review 確認。

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
| v1.2 | 2026-11-18 | dev 驗收腳本的細節（第七節第 4 點；使用者裁決）：D1 直接寫任務、直接打 python-server，backend 的提交流程這次不驗；D2 持鎖改鎖 `backtest_trades`、同步點用 `pg_blocking_pids()` 核對本輪持鎖 PID、測試任務至少要有一筆交易；D3 輪詢間隔不改，race 要有兩條路徑都嘗試認領同一筆的證據才算通過；D4 schema 情境用臨時 DB 加手動 `ALTER`，只驗欄位出現後能繼續啟動，臨時 server 不發布 port、先停 container 與連線再刪 DB。另：回應遺失改用不讀回應的 socket client；收尾改成先停止執行路徑、放鎖與清資料，最後依開始時的紀錄恢復服務；build 與持鎖連線受總剩餘時間約束；前綴用字面比對。5.1 補認領的 log。review [P2]×2：lost-response 也用明細表的鎖當同步點，確認 worker 三輪沒有接手才放鎖；收尾要先確認執行路徑已停止，才刪測試資料與恢復服務，未確認時保留資料、回傳非 0，socket client 與持鎖連線仍照常結束。 |
| v1.3 | 2026-11-18 | 實作時發現的既有 bug（使用者同意併入）：Go 的 `GetJob`／`ListJobs` 讀不到 `error` 為 NULL 的任務，`pending`／`running` 的 GET 回 404、列表回 500、取消回 404 → 兩個查詢改成 `COALESCE(error, '')`（5.2）；worker 在 PostgreSQL 上解析不了 JSONB 的 `symbols`、日期是 `date` 物件，每一筆都失敗 → 只有字串才 `json.loads`、日期轉成字串（5.1）。`UpdateJobStatus` 沒有呼叫者，刪除。dev 驗收的資料：dev DB 原本沒有任何日 K，使用者同意從舊機最終 dump（SHA-256 `1d9495fd…`）只匯入 2330 的日 K（4,930 筆），⛔ 不匯入其他表；預設策略 `breakout_swing_atr_v1`（`breakout_v1` 有交易就失敗，另立 I-135）。 |
| v1.3（review） | 2026-11-18 | review [P2]×3：臨時 server 依名稱移除並確認、確認不了就⛔ 不刪臨時 DB；schema 等待的探測受剩餘時間限制、回來後再核對期限；dev 腳本的收尾共用截止時間、用完⛔ 不再給預算並列出未完成的項目。 |
