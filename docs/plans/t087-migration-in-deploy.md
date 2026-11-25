# T-087 計畫書 v1.2：資料庫 migration 從 backend 啟動移到部署流程

> 狀態：v1.2（2026-11-25，併入 v1.1 review：外層逾時改成獨立的故障注入、還原後用應用帳號驗證權限），待複審。對應：`docs/todo.md` T-087。
> 裁決（2026-11-25）：Q1 採「停機中正式套用＋停機前試跑」；Q2 live 啟動只做唯讀檢查、dev 明確啟用自動套用；Q3 python 維持等欄位；
> Q4 先完成 T-084，兩案都通過驗收再合併成一次 live `deploy.sh` 換檔（T-087 的復原演練延後時，T-084 可以單獨上線）。
> 時程：只寫計畫；實作排在 11-30（I-141 上 live）之後、T-084 之後。

## 一、目標與不做的範圍

**目標**

1. **migration 模式**：同一支 backend binary 加 migration 模式（只載入資料庫設定、連 DB、做完就結束）；`deploy.sh` 用**新 image** 的一次性容器呼叫它。
2. **停機前試跑**：`down` 之前，在拋棄式 PostgreSQL 上用 live 的 schema **加上 `goose_db_version` 的紀錄**試跑待套用的 migration；失敗就中止部署，原服務照常運作。
3. **停機中正式套用**：`down` 之後、`up` 之前，用同一個新 image 正式套用；套用前核對 live 的版本紀錄仍等於試跑的基準；有鎖等待與整體執行時間的上限。
4. **live 啟動只讀檢查**：backend 啟動時在唯讀交易裡讀版本表，**image 內建的每一支 migration 都已套用**才啟動；DB 多出較新的版本可以接受。
5. **dev 與測試照舊一鍵完成**：dev compose 明確開啟自動套用；SQLite 單元測試、`test-postgres-migrations.sh`、`test-mysql-migrations.sh` 沿用現在的入口。
6. **復原分支寫清楚並演練**（第五節）；部署腳本⛔ 不自動跑 `goose down`。

**不做**

* ⛔ 不做停機前的線上套用（Q1 的 (b)）：首版正式套用一律在停機中。
* ⛔ 不改 python 的等待條件（Q3）：繼續等 077／078 的欄位，保留 dev 自動套用時的啟動容錯。
* ⛔ 不改既有 migration 的內容、⛔ 不重排編號。
* ⛔ 不做自動回滾：失敗時停下並印出下一步，由人照演練過的程序處理（跟 I-132 的 `deploy.sh` 一致）。
* ⛔ 不處理 MySQL 的部署（從未部署過；三種 migration 照舊同步維護）。

## 二、現況（2026-11-25 讀碼）

* backend 啟動順序：載入設定 → 設定契約 → 連 DB → `database.RunMigrations`（goose `UpContext`，成功後用 `GetDBVersion` 記 log）→ Redis 探測 → 其餘。
  migration 失敗就 `os.Exit(1)`；`deploy.sh` 在 `down` 之前⛔ 沒有辦法知道會不會失敗。
* `RunMigrations` 依 driver 選 embed 的目錄（sqlite／mysql／postgres），三者都用 goose v3.21.1。
  ⚠️ `GetDBVersion` 在版本表不存在時會走建表的流程，⛔ 不能當唯讀檢查。
* goose 的交易邊界是**單一 migration 檔**：前幾支可能已經提交，後面那支失敗才回滾它自己；非交易式（`NO TRANSACTION`）的檔可能留下部分變更。
  目前 postgres 的 80 支（001～080）⛔ 沒有任何一支是非交易式。
* 鎖等待：075 在檔內 `SET LOCAL lock_timeout = '5s'`（ALTER 要 ACCESS EXCLUSIVE，卡在盤中寫入時會排隊擋住後面的查詢）；其他檔⛔ 沒有設定。
* `-check-config` 是既有的「同一支 binary 的另一種模式」先例：只載入設定與契約、⛔ 不連 DB；`deploy.sh` 先 grep 執行檔確認新 image 認得旗標，才執行它
  （舊 binary ⛔ 不解析參數，會直接啟動完整的 backend）。
* python 兩個服務在接任務之前等 077、078 的欄位（`_schema_ready`），所以⛔ 不依賴 backend 與 python 的啟動先後。
* 舊 image（T-087 之前）啟動時照樣自動套用：DB 已經是較新的版本時，舊 binary 的 embed 裡每一支都已套用，goose ⛔ 沒有事可做——
  回到舊 image 時⛔ 不會碰 schema（第六節要實際演練確認）。
* live 的 DB 備份：`docs/` ⛔ 沒有例行的 live DB 備份程序；dump 只在搬遷與取證時做過。

## 三、設計

### D1：migration 模式（同一支 binary）

* 旗標 `-migrate=status` 與 `-migrate=up`（字串旗標；其他值直接結束、非 0）。
* 只載入設定裡的 `database` 區段並檢查它（driver、DSN 有值）；⛔ 不跑完整的設定契約（`deploy.sh` 第 5 步已經用新 image 跑過）、⛔ 不連 Redis、⛔ 不啟動排程與 HTTP。
* **`status`（唯讀）**：在唯讀交易裡讀 `goose_db_version`（見 D3 的讀法），跟 embed 的 migration 清單比對，輸出：
  * `required`：embed 的版本數與最大版本；
  * `applied`：DB 已套用的版本數與最大版本；
  * `pending`：embed 有、DB 沒有的版本清單（**含中間缺號**）；
  * `extra`：DB 有、embed 沒有的版本清單；
  * `fingerprint`：版本表「已套用集合」的 SHA-256（排序後的版本清單），給試跑與正式套用比對基準。
  輸出只有版本號與計數，⛔ 不帶 DSN 或錯誤原文；結束碼：0 成功、2 版本表不存在、1 其他錯誤。
* **`up`**：
  * 先做一次 `status`，印出基準；`pending` 是空的就直接結束（0）。
  * 連線帶 `lock_timeout`（預設 5 秒）與 `statement_timeout`（預設 10 分鐘），整體用 context 的期限（預設 15 分鐘）；三個值都可以用旗標覆寫，⛔ 不新增設定鍵（Q-B 裁決的首版預設）。
  * 連線的 `application_name` 帶**本次執行的識別**：`trading-migrator-<run_id>`（`run_id` 由 `deploy.sh` 產生、跟具名容器用同一個時間戳，經 `-run-id` 旗標傳入），
    讓 `pg_stat_activity` 只認得出**這一次**的 migrator 工作（D7 的停止確認用，⛔ 不會取消別次或別人的工作）。
  * 逐支套用（goose 的 `UpByOne` 迴圈），每支前後各記一行「版本、開始、結束、耗時」；任何一支失敗就停下、⛔ 不繼續下一支，最後再做一次 `status` 印出實際狀態。
  * 結束碼：0 全部成功、3 有一支失敗（前面的已提交）、4 逾時、1 其他。
  * `-expect-fingerprint=…`（選填）：套用前的 `fingerprint` 跟它不同就⛔ 不套用、結束碼 5（D4 正式套用時用）。

### D2：啟動時只讀檢查（live），dev 明確啟用自動套用

* 新設定鍵 `database.auto_migrate`（環境變數 `DATABASE_AUTO_MIGRATE`），**預設 false**。
  * `true`：照現在的 `RunMigrations`（dev compose 明確設 `true`）。
  * `false`：改成**唯讀檢查**：在唯讀交易裡讀版本表；版本表不存在、查詢失敗、或 embed 的任何一支⛔ 沒有套用 → log 記「缺哪幾支」（只有版本號）並結束、非 0。
    確認必要的都已套用之後，才接受 DB 有額外的新版本（記一行 Info）。
* 設定契約：production（`IsProduction()`：`APP_ENV` 是空的或 `production`）且 `auto_migrate=true` → 違反契約、啟動就擋（live ⛔ 不能靠設定錯誤退回自動套用）。
* 受影響的入口：
  * `docker-compose.dev.yml` 的 backend 加 `DATABASE_AUTO_MIGRATE: "true"`；dev smoke 腳本都經過它，照舊一鍵。
  * `docker-compose.yml`（live 範本）⛔ 不設（預設 false）。
  * Go 測試（SQLite、postgres、mysql 的 migration 測試）直接呼叫 `RunMigrations`，⛔ 不經過啟動路徑，照舊。
* python：維持等欄位（Q3）。dev 自動套用時 python 可能比 backend 先起來，照舊等到欄位出現；live 在 `up` 之前已經套用完，python 第一次探測就通過。

### D3：唯讀的版本讀法（⛔ 不用 `GetDBVersion`）

* 先確認表存在：postgres 查 `to_regclass('goose_db_version')`、SQLite 查 `sqlite_master`、MySQL 查 `information_schema.tables`；不存在 → 「版本表不存在」。
* 再讀 `SELECT version_id, is_applied FROM goose_db_version ORDER BY id`，**每個 `version_id` 取最後一列**的 `is_applied` 決定「已套用集合」：
  * goose v3.21.1 的 SQL migration 跑 down 時，會呼叫 `DeleteVersion` **刪掉**那個版本的紀錄——撤回的版本在表裡就⛔ 沒有列，自然不在已套用集合；
  * `is_applied=false` 的列是防禦性處理（舊版 goose 或手動寫入可能留下），同樣視為未套用。
* postgres 用 `BEGIN READ ONLY`；SQLite／MySQL 在測試裡驗「讀完之後表與內容都沒變」。
* 「必要清單」由 embed 目錄的檔名解析（跟 goose 用同一種解析），⛔ 不手寫版本號。

### D4：`deploy.sh` 的新流程

```text
1 preflight（含 T-084 的第一次回測檢查）→ 2 JWT → 3 git pull → 4 build → 5 新 image 的 -check-config
→ 5a 確認新 image 認得 -migrate（grep 執行檔，⛔ 不執行；比照 -check-config）
→ 5b 新 image 跑 -migrate=status 對 live DB（唯讀）：記下 base fingerprint 與 pending
→ 5c pending 不是空的：停機前試跑（D5）；失敗 → 中止，原服務照常運作
→ 5d T-084 的第二次回測檢查
→ 6 down
→ 6a pending 不是空的：備份（D6）→ 新 image 跑 -migrate=up -expect-fingerprint=base（具名容器，見 D7）
→ 6a′ 不論成功失敗：停止確認（D7）——容器已退出、PostgreSQL 上⛔ 沒有 migrator 的工作
→ 6b 再跑一次 -migrate=status：pending 必須是空的
→ 7 up -d --no-build → 8 readiness
```

* pending 是空的（多數部署）：5c、6a 整段跳過，流程跟現在一樣，只多 5a、5b 兩個唯讀步驟。
* 6a、6b 失敗：⛔ 不 `up`、⛔ 不跑 `goose down`；印出 `-migrate=status` 的結果與第五節的對應分支，結束、非 0。
  （`build` 之後 compose 的 `latest` 已經指向新 image，舊 image 只能用部署前打的回滾標籤找回；自動 `up` 會啟動新 image。）

### D5：停機前試跑

* **拋棄式 PostgreSQL**（Q-C 裁決）：跟 live 同一個 PostgreSQL 主版本的 image，在 live 主機起一個只給這次試跑用的 container（⛔ 不對外開 port、暫時的帳密、結束就刪 container 與 volume）。
  ⛔ 不在 live 的 PostgreSQL 裡建臨時 database（不需要 CREATEDB 權限，也⛔ 不跟正式資料共用實例）。
* **獨立網路**：試跑用一個新建的 docker 網路，只有拋棄式 PostgreSQL 與試跑的 migrator 在上面；試跑的 migrator 用 `docker run --network 試跑網路`（⛔ 不用 compose run，
  compose 會把它接上 live 的網路），DSN 只帶臨時庫——migrator 實體上連不到 live 的 PostgreSQL。
* **帶入的內容**：
  1. live 的 schema：`pg_dump --schema-only --no-owner --no-acl`；
  2. **`goose_db_version` 的紀錄**：`pg_dump --data-only -t goose_db_version`（`--schema-only` 只會帶到版本表的結構，⛔ 沒有紀錄；少了紀錄，goose 會在已經有物件的庫上從第一支重跑）；
  兩份都在 live 的 postgres container 裡用唯讀的方式匯出，直接接到拋棄式 container 還原，⛔ 不落地成檔案。
* **試跑前核對目標**：
  * 還原後在臨時庫建一張標記表（`t087_trial_target`，只存在臨時庫）；試跑的 migrator 啟動前，用同一個 DSN 確認標記表存在——⛔ 不存在就中止（DSN 指錯了）；
  * 在臨時庫跑 `-migrate=status`，`fingerprint` 要等於 5b 從 live 讀到的 base；不同就中止（匯出到一半有變動）。
* **試跑**：同一個新 image 跑 `-migrate=up` 對拋棄式庫；全部成功、之後 `status` 的 pending 是空的才算通過。
* **試跑的承諾只到 SQL、物件依賴與套用順序**：空表⛔ 不能驗證既有資料符不符合新的約束，也⛔ 不能量出大表改寫的耗時；這些靠 D1 的上限與第五節的復原分支。
* 試跑的 log 只留版本號、耗時與結果；拋棄式 container、試跑的 migrator、試跑網路不論成功失敗都刪（`trap`），刪完確認三者都已不存在。

### D6：停機中的備份

* pending 不是空的時（Q-A 裁決），`down` 之後、正式套用之前，對 live DB 做一份 `pg_dump -Fc` 到 live 主機的備份目錄（權限 700，檔名帶日期與 base fingerprint 前綴），
  完成後用 `pg_restore -l` 確認**檔案讀得出來**；**能不能還原**由第六節的演練實際驗證，⛔ 不以 `-l` 代替。
* 備份失敗 → ⛔ 不套用，停下（服務維持停止，照第五節「停機前就停下」的方式恢復舊服務）。
* 這段會拉長停機時間：live DB 的實際 dump 與還原耗時在演練時用作業隔離 volume 上的 live 複本量（第六節），寫進部署手冊；
  **部署窗口＝備份＋套用上限（15 分鐘）＋停止確認＋可能的還原**，⛔ 不能只算套用的時間。

### D7：正式 migrator 的具名容器與停止確認

* 正式套用用**具名**的一次性容器：`compose run --rm --no-deps -T --name stock_trading-migrator-<run_id> backend ./trading-backend -migrate=up -run-id=<run_id> …`；
  `run_id` 在 6a 開始前產生並印出，容器名稱與 `application_name` 共用它。
* 外層用 `timeout -k 10 <整體上限＋60 秒>` 包住。⚠️ `timeout` 管的是外層的 docker CLI：它被砍掉⛔ 不代表容器已經停止、更⛔ 不代表 PostgreSQL 上的工作已經結束。
* **停止確認**（6a′；成功、失敗、逾時都做；第五節的重試、還原、啟動舊服務之前也一定先做）：
  1. **容器**：`docker inspect` 那個名稱——還在跑就 `docker stop -t 30`，再不停就 `docker kill`；最後確認容器⛔ 不存在或狀態是 exited（每個 docker 呼叫都包 `timeout -k`）。
  2. **DB 工作**：唯讀查 `pg_stat_activity` 裡 `application_name = 'trading-migrator-<run_id>'` **而且** `datname` 是部署目標的 database 的連線數
     （只看筆數與狀態、⛔ 不印查詢內容）；不是 0 → 等最多 60 秒 → 仍不是 0 就對**這些 pid** `pg_cancel_backend`（取消目前的語句、交易回滾）→ 再等 30 秒 →
     仍不是 0 就對這些 pid `pg_terminate_backend` → 再確認是 0。⛔ 不對其他 `application_name`、其他 database 的連線做任何事。
  3. 三步任何一步確認不了（docker 沒回應、查詢失敗、最後仍不是 0）→ 停下、⛔ 不往下做任何復原動作，印出「需要人工確認 migrator 已停止」與容器名稱。
* 停止確認做成 `deploy.sh` 的函式，同時提供手動執行的方式（部署手冊的復原步驟會呼叫它）。
* 備份的保留：留到下一次有 migration 的部署成功、而且回滾窗口結束；空間先在 live 唯讀盤點時確認。

## 四、受影響的檔案、模組與資料流

* backend
  * `cmd/server/main.go`：`-migrate` 旗標的分流；啟動時依 `auto_migrate` 走 `RunMigrations` 或唯讀檢查。
  * `internal/database`：唯讀的版本讀法（D3）、embed 清單解析、`status`／`up` 的實作（`UpByOne` 迴圈、期限、指紋）。
  * `internal/config`：`database.auto_migrate` 的預設值與 `SetDefault`、設定契約的 production 規則。
  * 測試：三種 DB 的唯讀檢查、`status`／`up` 的結束碼與輸出、契約規則。
* `deploy.sh`（範本）：D4～D6；`scripts/test-deploy-sh.sh` 補對應的假 docker 案例。
* `docker-compose.dev.yml`：`DATABASE_AUTO_MIGRATE: "true"`。`docker-compose.yml`、`.env.example`：⛔ 不設（註解說明預設 false）。
* 文件：`development-workflow.md`（部署流程、試跑、備份、復原）、`architecture.md`（啟動順序、設定契約）、`database-schema.md`（migration 的套用方式、「先加後刪」與回滾窗口）。
* 資料流：live 的 schema 變更從「新 backend 啟動時」移到「`down` 之後、`up` 之前」；⛔ 沒有新的資料表或 API。

## 五、復原分支（裁決）

| 失敗點 | 處理 |
|---|---|
| 停機前（5a～5d：旗標確認、status、試跑、第二次回測檢查） | 中止部署，原服務照常運作；⛔ 沒有碰 live 的 schema |
| `down` 之後、套用之前（基準不符、備份失敗） | ⛔ 沒有碰 schema（migrator 還沒啟動）：用部署前的回滾標籤把舊 image 換回 `latest`，`up -d --no-build`（照部署手冊的 R1、R2） |
| 套用到一半失敗或逾時（6a 結束碼 3、4，或外層逾時） | 1. **停止確認**（D7）通過——容器已退出、PostgreSQL 上⛔ 沒有 migrator 的工作；2. 用 `-migrate=status` 查最後成功的版本，並對照實際 schema（失敗那支要的物件在不在）；3. **舊 image 確認相容**（已提交的那幾支都是「先加」的變更）才換回舊 image 啟動；4. 不相容、或 schema 有部分變更時，照演練過的程序從 D6 的備份**還原到乾淨的 DB**（見下）再啟動舊 image |
| 全部套用成功，新服務啟動或 readiness 失敗 | 優先換回**已驗證相容**的舊 image、保留新 schema（回滾窗口內舊版必須能讀寫新 schema）；要還原 DB 時，先確認新版啟動後有沒有寫入（`job_runs` 等的新紀錄），再決定保留或補回 |

* 部署腳本⛔ 不自動跑 `goose down`；也⛔ 不自動 `up`（理由見 D4）。
* **還原到乾淨的 DB**（⛔ 不在套用過的庫上反向修補，避免部分套用新增的物件殘留）：
  1. 停止確認（D7）通過，所有應用服務都是停的；
  2. 在 live 的 PostgreSQL 建一個新的 database（`trading_restore_<時間戳>`，**owner 是正式應用帳號 `trading_user`**），從 D6 的備份還原：
     * **還原角色**：用 `pg_restore --role=trading_user --no-owner --no-acl --exit-on-error` 以應用帳號的身分建立物件，所有表、sequence、index 都由 `trading_user` 擁有，
       跟現在 live 的擁有關係一致（`--no-owner` 會讓物件歸還原時的角色所有、`--no-acl` 會略過原有授權，所以角色要指定成應用帳號）；
       直接用 `trading_user` 連線還原時物件自然歸它所有；用管理帳號連線時才需要 `--role`，而且管理帳號必須是 `trading_user` 角色的成員（盤點時確認）；
     * 現有 live 的授權若⛔ 不只是 owner（例如另有唯讀帳號的 GRANT），先在盤點時列出，還原後照清單補上；
  3. 核對：
     * `-migrate=status` 的 fingerprint 等於部署時的 base、逐表筆數等於備份前記下的數字；
     * 擁有者：還原庫的 schema 物件（表、sequence、view）全部由 `trading_user` 擁有（查 `pg_class`／`pg_namespace`，列出例外，例外數必須是 0）；
     * **改名之前，用正式應用帳號 `trading_user` 連還原庫實測權限**：每張應用表各做一次 `SELECT … LIMIT 1`；在一個交易裡對代表性的表（`job_runs`、`backtest_jobs`、
       `position_analyses` 等，清單在演練時定）`INSERT`、`UPDATE`、`DELETE` 一筆後 `ROLLBACK`；每個 sequence `nextval` 一次（sequence 前進⛔ 不影響正確性，只會跳號）；
       任何一項失敗就⛔ 不改名、先修權限；
  4. 切換：確認兩個 database 都⛔ 沒有連線後，`trading` 改名成 `trading_failed_<時間戳>`、`trading_restore_<時間戳>` 改名成 `trading`；
  5. 用回滾標籤啟動舊 image，跑部署手冊的部署後確認；`trading_failed_<時間戳>` 留到原因查清楚、確認不需要之後才刪。
  需要的權限（建 database、改名）在開工前的 live 盤點確認；權限不夠時改用 live 專人的管理帳號，程序在演練時一併確定。
* **回滾窗口**：有 migration 的部署之後，到下一次確認不再回滾之前，舊版必須能讀寫新 schema。拆除欄位、切斷舊的寫法要等回滾窗口結束後的另一次 migration（「先加後刪」）。
  每支新 migration 在計畫書裡要寫明它屬於「先加」還是「後刪」。

## 六、測試與驗證

**Go（`backend/scripts/test.sh`）**

* 唯讀檢查（SQLite＋postgres；MySQL 用 `test-mysql-migrations.sh`）：版本表不存在 → 拒絕、⛔ 沒有建表；缺中間一支（例如刪掉 079 的那一列）→ 拒絕並列出 079；
  全部都在 → 通過；多一支 embed 沒有的版本 → 通過並記 Info；**實際跑一次 goose down**（撤回最後一支，goose 刪掉它的紀錄）→ 拒絕並列出那一支；
  某版本最後一列是 `is_applied=false`（防禦性，手動寫入）→ 視為未套用；檢查前後版本表的內容與結構都沒變。
* `-migrate=status`：輸出的 pending（含中間缺號）、extra、fingerprint；結束碼 0／2／1。
* `-migrate=up`：全部成功 → 0；第二支故意失敗（測試用的 embed 目錄）→ 結束碼 3、第一支已提交、第二支沒有；
  另一個連線持有表鎖 → 在 `lock_timeout` 內失敗、⛔ 不無限等；整體期限到 → 結束碼 4；`-expect-fingerprint` 不符 → 5、⛔ 沒有套用任何一支。
* 設定契約：production＋`auto_migrate=true` → 違反；dev 預設 false 時，啟動走唯讀檢查。
* 反證：唯讀檢查改回只看最大版本 → 「缺中間一支」的案例要失敗；拿掉 `-expect-fingerprint` 的比對 → 不符的案例要失敗。

**`scripts/test-deploy-sh.sh`（假 docker）**

* pending 空：⛔ 沒有試跑、⛔ 沒有備份、⛔ 沒有 `-migrate=up`；順序照舊。
* pending 不空、試跑成功：試跑在 `down` 之前、備份與正式套用在 `down` 之後 `up` 之前、正式套用帶 `-expect-fingerprint`；之後才 `up`。
* 試跑失敗、base 不符、新 image 不認得 `-migrate` → 停在 `down` 之前。
* 備份失敗、正式套用結束碼 3／4／5 → ⛔ 沒有 `up`、⛔ 沒有 `goose down`；訊息有對應的下一步。
* 拋棄式 container、試跑 migrator、試跑網路在成功與失敗兩種路徑都被刪除；試跑的 migrator 是 `docker run --network 試跑網路`、⛔ 不是 compose run。
* 試跑目標的標記表不存在 → 停在 `down` 之前。
* 正式套用的容器有固定格式的名稱；外層 timeout 砍掉 docker CLI 之後，假 docker 的容器仍「在跑」→ 停止確認會 stop／kill 它，確認退出後才印復原訊息；
  `pg_stat_activity` 仍有 migrator 的連線 → 依序 cancel、terminate；仍然不是 0 → 停下、印「需要人工確認」，⛔ 沒有任何復原動作。

**dev 演練（上 live 之前必做，記錄耗時與結果）**

在 dev 用拋棄式的 PostgreSQL 與作業隔離 volume 上的 live 複本做，⛔ 不碰一般的 dev DB：

1. **前一支成功、下一支失敗（依資料觸發）**：在演練用的分支加兩支測試 migration（⛔ 不 commit 到 init）：
   * 第一支：新增一個可為 NULL 的欄位（「先加」，舊版相容）；
   * 第二支：加一個 `CHECK` 約束，**只有正式演練庫裡事先放的測試資料會違反**（試跑的臨時庫只有 schema、沒有資料，所以試跑會通過）。
   跑一次完整的 `deploy.sh`（dev 的目錄配置，正式目標是演練庫）→ 要證明：
   * 試跑**通過**（5c 沒有擋下）；
   * 正式套用時第一支**已提交**、第二支失敗（結束碼 3），`status` 顯示第一支在、第二支不在；
   * 停止確認通過；腳本⛔ 沒有 `up`，服務維持停止；
   * 照第五節換回舊 image 啟動成功、舊版能讀寫（第一支加的欄位不影響它）。
2. **外層逾時（獨立的故障注入）**：只縮短 migrator 的整體上限時，會先收到 migrator 自己的逾時（結束碼 4），驗不到「docker CLI 已退出、容器仍在跑」。所以演練另外注入：
   * migrator 的整體上限、語句上限、鎖等待都維持很長（例如 1 小時），第二支是會長時間等鎖的變更、另一個連線持有那張表的鎖；
   * **只把外層的 timeout 調短**（演練用的 `deploy.sh` 參數）→ 外層 timeout 砍掉 docker CLI，容器與 DB 連線都還在；
   * 要證明：停止確認找到那個具名容器並把它停掉、查到 `trading-migrator-<run_id>` 在目標 DB 的連線、依序 cancel／terminate 到歸零；**歸零之前**腳本⛔ 沒有任何復原動作；
     持有鎖的那個連線與同時存在的另一個 migrator 識別的連線（模擬別次）⛔ 沒有被取消；那支的交易已回滾、`status` 顯示它沒有套用。
3. **schema 升級成功、服務換版失敗**：migration 成功、讓新 backend 的 readiness 失敗（演練用的故障設定）→ 換回舊 image、保留新 schema，舊版能讀寫（含 python 兩個服務）。
4. **備份可還原、切回舊服務**：接著第 1 項的狀態（第一支已提交），照第五節「還原到乾淨的 DB」做完五步：新 database 從 D6 的備份還原、fingerprint 等於 base、逐表筆數相同、
   改名切換、用舊 image 啟動並通過部署後確認；確認第一支新增的欄位在切換後的 `trading` 裡**不存在**（⛔ 沒有殘留）。
   還原與權限驗證都用演練庫的正式應用帳號做（演練庫的帳號配置比照 live：應用帳號擁有物件、管理帳號另一個）；另做一次反證——故意用管理帳號還原、⛔ 不帶 `--role`，
   權限實測要失敗，證明這一步驗得出問題。另外用作業隔離 volume 上的 live 複本量 dump 與還原的耗時。
5. **舊 image 碰到較新的 schema**：T-087 之前的 image 啟動時⛔ 沒有改動 schema（自動套用無事可做）；T-087 之後的舊 image 啟動時唯讀檢查通過（DB 多出的版本被接受）。

## 七、上 live

* 前提：T-084 已實作並通過 review；T-087 的演練全部通過。兩者合併成**一次** live `deploy.sh` 換檔（照 I-132「先試跑、再換檔」，T-084 第八節的核對一併做）。
  T-087 的演練延後時，T-084 先單獨換檔。
* 換檔之後的第一次部署：
  * 先部署 T-087 的程式本身（新 image 有 `-migrate`；DB 已是最新、pending 空，所以只會跑 5a、5b）；
  * backend 改成唯讀檢查啟動，readiness 通過。
* 實作前的 live 唯讀盤點（指令另給）：PostgreSQL 的主版本與 image（live 主機上有沒有同版本的 image）、主機的可用記憶體與磁碟（試跑容器、備份目錄）、DB 大小、
  `goose_db_version` 的已套用集合跟 init 的 embed 清單一致、目前有沒有其他長時間連線（只看筆數與 `application_name`）、`trading_user` 能不能建 database 與改名（還原程序用）、
  **現有物件的擁有者與 owner 以外的授權**（還原後要一致；只列角色與物件數，⛔ 不列資料）。

## 八、完成後要歸檔的位置

* `docs/development-workflow.md`：「部署腳本：失敗即停」的新流程（試跑、備份、正式套用、復原分支）、`-migrate` 模式的用法、dev 的 `DATABASE_AUTO_MIGRATE`。
* `docs/architecture.md`：backend 的啟動順序（唯讀檢查）、設定契約的新規則。
* `docs/database-schema.md`：migration 的套用時機、「先加後刪」與回滾窗口、每支 migration 標明類型。
* `docs/todo.md` T-087 改成「已實作／待 review」；live 換檔、review 確認後收斂。

## 九、裁決（2026-11-25）

* **Q-A 備份**：採停機後備份（D6）。`pg_restore -l` 只確認備份檔讀得出來，還原能力由演練驗證；演練寫清楚還原到乾淨的 DB、核對版本、切回舊服務（第五節、第六節第 4 項）。
* **Q-B 上限**：首版預設 `lock_timeout` 5 秒、單一語句 10 分鐘、整體 15 分鐘；整體上限到期後仍要完成停止確認（D7）；部署窗口另計備份與可能的還原時間。
* **Q-C 試跑環境**：live 主機上的隔離容器；開工前盤點 PostgreSQL image、記憶體與磁碟；試跑用獨立網路、migrator 明確指向臨時庫、試跑前核對目標（D5）。

## 版本紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-25 | 初版：依 2026-11-25 裁決（Q1 停機中正式套用＋停機前試跑、Q2 唯讀檢查、Q3 python 不動、Q4 先 T-084）；試跑帶入 `goose_db_version` 的紀錄並核對指紋；復原分支四種；dev 演練四項；三項待裁決 |
| v1.1 | 2026-11-25 | 併入 v1 review：[1] 部分成功演練改成依資料觸發（第二支的約束只有正式演練庫的資料會違反），要證明試跑通過、正式套用第一支已提交、第二支失敗、腳本維持停機、能恢復舊服務；另加外層逾時演練；[2] 正式 migrator 用具名容器與 `application_name`，新增 D7 停止確認（容器退出、`pg_stat_activity` 歸零，必要時 cancel／terminate），重試、還原、啟動舊服務前一定先做；[3] 更正 goose down 會刪除版本紀錄，補實際跑 down 的測試；第九節三項依建議裁決：還原到乾淨的 DB 的五步程序、部署窗口計入備份與還原、試跑用獨立網路與目標標記表 |
| v1.2 | 2026-11-25 | 併入 v1.1 review：[1] 外層逾時改成獨立的故障注入（migrator 自己的上限拉長、只縮短外層 timeout），證明停止確認找到具名容器、目標 DB 上本次的連線歸零之前⛔ 沒有復原動作；`application_name` 帶本次的 `run_id`、停止確認只限目標 DB 與本次的 pid；[2] 還原角色定為 `trading_user`（`--role`、新庫的 owner），改名前用應用帳號實測讀、寫、sequence 權限與擁有者；盤點加現有擁有者與授權；演練加權限的反證 |
