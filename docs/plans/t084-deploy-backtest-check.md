# T-084 計畫書 v1.1：`deploy.sh` 的部署前檢查加上回測的 `pending`／`running`

> 狀態：v1.1（2026-11-25，併入 v1 review：兩次檢查的設計可採，補三點），待複審。對應：`docs/todo.md` T-084。
> 需求（2026-11-25 使用者確認）：把部署規範的人工「回測雙零檢查」做進 `deploy.sh`；查不到算失敗、⛔ 不留跳過的選項、DB 的 container 與帳號在腳本開頭設定。
> 時程：只寫計畫。實作與換上 live 都排在 I-139（11-28）與 I-141（11-30）上版之後——那兩次都用現在的 `deploy.sh`，⛔ 不在中間換。

## 一、目標與不做的範圍

**目標**

* `deploy.sh` 自己查 `backtest_jobs`：`pending` 或 `running` 有任何一筆就停下，印出兩個筆數，提示等回測結束再部署。
* 查兩次：
  1. **preflight**（`git pull` 之前）：最早擋下，什麼都還沒動；`./deploy.sh --preflight` 也跑這一次。
  2. **`down` 之前**（新 image 的設定檢查通過之後）：build 要好幾分鐘，這段期間舊的 worker 還在跑、會接走新送出的回測；`down` 前再查一次，有就停下（新 image 已經 build 好，但舊服務照常運作）。
* 查不到（container 不在、連不上、逾時、權限、輸出格式不對）一律算失敗、停下，訊息寫出是哪一類，⛔ 不帶原始錯誤。
* 唯讀連線：`default_transaction_read_only=on`，另帶 `statement_timeout`。
* 每一個 docker 呼叫（`inspect` 與 `exec`）都有硬逾時與強制終止上限，Docker daemon ⛔ 沒有回應時也不會一直卡住。

**不做**

* ⛔ 不阻止部署期間新送出的回測：第二次檢查到 `down` 之間仍有幾秒的空窗，部署規範的「從開始部署到部署後確認做完，⛔ 不送回測」照舊保留。
* ⛔ 不留跳過檢查的選項（使用者裁決）：真的要在有回測時部署，改用人工流程（手動 build、`down`、`up`），並先處理那些回測。
* ⛔ 不改 `deploy.sh` 其他步驟的行為與順序。
* ⛔ 不改 `dc.sh`。
* 在 live 換上新版之前，人工的雙零檢查照做（部署手冊 A 段）；換上之後手冊 A 段仍保留查詢當作紀錄，判定以 `deploy.sh` 為準。

## 二、現況（2026-11-25 讀碼）

* `deploy.sh` 範本（I-132）：preflight（git、檔案權限、`env_file`、Redis 的 `.env`）→ JWT 檢查 → `git pull --ff-only` → 逐一服務 build（舊服務還在跑）
  → 確認新 image 認得 `-check-config` → 用新 image 檢查設定 → `down` → `up -d --no-build` → readiness。`down` 之前的任何失敗都停下、⛔ 不碰服務；錯誤訊息⛔ 不帶機密。
* `--preflight` 只跑 preflight 與 JWT 檢查（加讀 `MemAvailable`），⛔ 不 pull、⛔ 不 build、⛔ 不 down；換新版 `deploy.sh` 之前先用暫存檔名試跑。
* 要填的值在開頭：`PROJECT_DIR`、`DEPLOY_BRANCH`、`REDIS_ENV_FILE`（範本是 `/path/to/…`），live 那份只換這三行，版本以 SHA-256 固定
  （`development-workflow.md`「部署腳本：失敗即停」的「live 的 `deploy.sh`」）。
* 腳本開頭 `exec </dev/null`：timeout 底下的 docker 讀終端機會被 SIGTTIN 停住（2026-11-16 的教訓）。
* live 的 DB 在另一個 stack 的 container（`postgres-postgres-1`），帳號 `trading_user`、database `trading`；live 的唯讀查詢一律用
  `docker exec -e PGOPTIONS="-c default_transaction_read_only=on …" postgres-postgres-1 psql -U trading_user -d trading -tAc …`，不需要密碼（container 內的本機連線）。
* `backtest_jobs.status` 的值：`pending`、`running`，結束是 `done`（成功）／`failed`（I-134 之後被中斷的也收成 `failed`）。worker 每隔 `WORKER_POLL_INTERVAL` 秒認領 `pending`。
* 測試：`scripts/test-deploy-sh.sh`——假的 `git`／`docker`／`sleep` 放在 PATH 前面，記錄呼叫順序，驗證每一種失敗的停止點；
  假 docker 目前只認 `inspect` 與 `compose …`，其他呼叫一律報錯。

## 三、設計

### D1：要填的值

開頭「要填的值」加三行：

```text
DB_CONTAINER="your-postgres-container"   # live：postgres-postgres-1
DB_USER="your-db-user"                   # live：trading_user
DB_NAME="trading"
BACKTEST_CHECK_TIMEOUT_SEC=15
```

* `DB_CONTAINER`、`DB_USER` 是範本的佔位值；preflight 一開始就檢查：還是佔位值或空字串 → 停下「先填 DB_CONTAINER／DB_USER」（⛔ 不呼叫 docker）。
* 值只允許 `[A-Za-z0-9_.-]`（container 名稱與 PG 帳號的合法字元的子集），其他字元 → 停下；避免把值拼進指令時出現意外。
* live 那份要換的行從三行變五行（加 `DB_CONTAINER`、`DB_USER`）；`DB_NAME` 兩邊都是 `trading`，⛔ 不用換。

### D2：查詢

```text
timeout -k 5 "$BACKTEST_CHECK_TIMEOUT_SEC" docker exec \
  -e PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=5000" \
  "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -X -v ON_ERROR_STOP=1 -qtAc \
  "SELECT count(*) FILTER (WHERE status = 'pending'), count(*) FILTER (WHERE status = 'running') FROM backtest_jobs"
```

* 輸出必須**剛好一行**、符合 `^[0-9]+\|[0-9]+$`，才取出兩個數字；其他任何輸出（空的、多行、文字）→ 「輸出格式不對」，停下。
* `docker exec` ⛔ 不帶 `-i`、⛔ 不讀 stdin（腳本開頭已經 `exec </dev/null`）。
* stderr 丟掉（`2>/dev/null`）：psql／docker 的原始錯誤⛔ 不印出來，只依結束碼分類。
* 查詢之前先確認 container 在跑：`timeout -k 5 "$BACKTEST_CHECK_TIMEOUT_SEC" docker inspect -f '{{.State.Running}}' "$DB_CONTAINER"`，⛔ 不是 `true` 就停下。
  **`inspect` 與 `exec` 都包 `timeout -k`**：到期送 TERM，再過 5 秒還沒結束就 KILL；daemon ⛔ 沒有回應時，單次檢查最多約 2 ×（15＋5）秒。
  逾時（結束碼 124，或 KILL 後的 137）一律歸成「逾時」。

### D3：判定與訊息

| 情況 | 判定 | 訊息（例） |
|---|---|---|
| `0\|0` | 通過 | `==> 回測：pending 0、running 0` |
| 任何一個 > 0 | 停下 | `backtest_jobs 有 pending 1、running 2，等回測結束再部署（⛔ 不留跳過的選項），停止部署（還沒 pull／還沒 down）` |
| `inspect` 或 `exec` 逾時（124／137） | 停下 | `查回測狀態逾時（inspect／exec，15 秒），停止部署（…）` |
| container 不在、⛔ 沒有在跑 | 停下 | `找不到在跑的 DB container DB_CONTAINER=…，停止部署（…）`（先用 `docker inspect -f '{{.State.Running}}'` 區分） |
| 其他非 0 結束碼 | 停下 | `查回測狀態失敗（結束碼 N；⛔ 不顯示原始錯誤，請手動用唯讀查詢確認），停止部署（…）` |
| 輸出格式不對 | 停下 | `查回測狀態的輸出格式不對，停止部署（…）` |

* 「還沒 pull」或「還沒 down」依呼叫的位置帶出，跟既有的訊息一致。
* 第二次檢查失敗時另外說明「新 image 已經 build 好、舊服務照常運作；回測結束後重跑 `./deploy.sh`」。
* 兩次檢查用同一個函式（`check_backtests 階段名`），⛔ 不寫兩份。

### D4：放的位置

* 第一次：preflight 的最後（`env_file` 與 Redis `.env` 檢查之後、JWT 檢查之前），所以 `--preflight` 也會跑到。
* 第二次：第 5 步（用新 image 檢查設定）通過之後、第 6 步的 `down` 之前。
* 檔頭的流程說明、`--preflight` 的說明一起更新。

## 四、受影響的檔案與資料流

* `deploy.sh`（範本）：D1～D4。
* `scripts/test-deploy-sh.sh`：假 docker 加 `exec`（只認 D2 的形式，其他照舊報錯）與兩次檢查的計數；新案例見第六節。
* `docs/development-workflow.md`：「部署腳本：失敗即停」的流程、「部署前的回測雙零檢查」（改成由 `deploy.sh` 判定，人工查詢保留為紀錄）、「live 的 `deploy.sh`」（換的行從三行變五行、新的 SHA-256）。
* 資料流：⛔ 沒有寫入；每次部署對 live DB 多兩次唯讀查詢。⛔ 沒有 schema、API、設定鍵的變更。

## 五、主要風險與回滾

| 風險 | 處理 |
|---|---|
| live 換上後第一次部署才發現連不上 DB | 換上之前先用暫存檔名 `--preflight` 試跑（第一次檢查就在 preflight 裡），連不上當場就看得到 |
| 填到另一個 PostgreSQL，查了錯的 DB 卻回 `0\|0` | 換檔前唯讀核對三個服務的 DSN（host、database）與實際查詢的指紋（第八節第 2 步）；不一致⛔ 不換檔 |
| 填錯 container 名稱，誤擋部署 | 訊息直接寫出 `DB_CONTAINER` 的值與「找不到在跑的 container」；改正後重跑 |
| DB 或 Docker daemon 卡住讓部署一直等 | `inspect`、`exec` 都包 `timeout -k 5 15`，另有 `statement_timeout` 5 秒 |
| 查詢帶出機密 | 只印兩個數字與分類；stderr 丟掉；帳號名稱⛔ 不是機密，但訊息也只印 `DB_CONTAINER` |
| 第二次檢查到 `down` 之間仍有空窗 | 人工規範「部署期間不送回測」保留；`down` 後 I-134 的回收會把被中斷的任務收成 `failed`，⛔ 不會停在 `running` |
| 回滾 | 換回備份的舊 `deploy.sh`（換上時留 `deploy.sh.bak-日期`，SHA-256 記在文件）；⛔ 沒有其他狀態 |

## 六、測試與驗證

**`scripts/test-deploy-sh.sh`（假 docker）**

* 正常：兩次都 `0|0` → 部署完成；呼叫紀錄裡第一次檢查在 `git pull` 之前、第二次在 `-check-config` 之後與 `compose down` 之前；兩次的 `PGOPTIONS` 都有 `default_transaction_read_only=on`。
* 第一次擋下：`1|0`、`0|2` → 非 0、⛔ 沒有 `git pull`、build、down；訊息有兩個筆數。
* 第二次擋下：第一次 `0|0`、第二次 `0|1` → 非 0；有 build 與設定檢查、⛔ 沒有 down／up；訊息有「新 image 已經 build 好」。
* `--preflight`：`0|0` → 通過、只查一次、⛔ 沒有 pull；`1|0` → 非 0。
* 查不到：container ⛔ 沒有在跑、`exec` 非 0、`inspect` 逾時、`exec` 逾時（假 docker 延遲超過上限）、輸出是空的、兩行、`abc`、`1|x` → 都停在 pull 之前，各自的訊息。
* **第二次查詢逾時**：第一次 `0|0`、第二次 `inspect` 或 `exec` 卡住 → 在上限內回傳非 0、⛔ 沒有 down／up；假 docker 的延遲程序在結束時⛔ 沒有殘留。
* 強制終止：假 docker 忽略 TERM（模擬卡死的 daemon 呼叫）→ 仍在 `逾時＋5 秒` 內結束、判成逾時。
* 呼叫紀錄裡每一個 `docker inspect`／`docker exec`（DB 檢查）前面都有 `timeout -k`。
* 假 docker 的 `exec` 往 stderr 印一個標記字串 → 輸出⛔ 不能出現它。
* 佔位值沒填、值有不允許的字元 → 停下，⛔ 沒有任何 docker 呼叫。
* 既有案例全部照舊通過（假 docker 的預設是 `0|0`）。
* 反證：拿掉第二次檢查 → 「第二次擋下」失敗；把查不到改成放行 → 「查不到」的案例失敗。

**dev 實測（⛔ 不碰 live）**

* **SQL 實測一律用拋棄式的 PostgreSQL**（比照 `scripts/test-postgres-migrations.sh`：獨立的 container 與 volume、跑完 migration、結束就刪），⛔ 不在共用的 dev DB 插入任務——
  dev 的 worker 會先認領 `pending`（預期的 `1|0` 變成 `0|1`），還可能留下回測結果與明細。
  從 `deploy.sh` 取出 D2 的 SQL 字串（⛔ 不另外手寫一份）執行：空表 → `0|0`；插入一筆 `pending` → `1|0`；再插入兩筆 `running` → `1|2`；`done`、`failed` ⛔ 不計入；
  同樣的 `PGOPTIONS` 下嘗試寫入會被唯讀擋下。
* `bash -n deploy.sh`；在 dev 用跟 live 相同的目錄配置（暫存的 scripts 目錄、`DB_CONTAINER` 指向上面那個拋棄式 PostgreSQL）跑 `--preflight`：通過、印出 `pending 0、running 0`；
  把 `DB_CONTAINER` 改成不存在的名稱 → 停下。

**live 換上（第八節）**：暫存檔名 `--preflight` 通過，另一份故意填錯 `DB_CONTAINER` 的暫存檔 `--preflight` 要停下。

## 七、完成後要歸檔的位置

* `docs/development-workflow.md`：「部署腳本：失敗即停」（流程、兩次檢查、訊息分類、⛔ 不留跳過的選項）、「部署前的回測雙零檢查」（由 `deploy.sh` 判定；部署期間不送回測的規範保留）、
  「live 的 `deploy.sh`」（五行、新的 SHA-256、換上日期）。
* `docs/todo.md` T-084 改成「已實作／待 review」；live 換上、review 確認後收斂。

## 八、上 live（換 `deploy.sh`）

* 時間：I-141 上 live（11-30）之後，任何一個⛔ 沒有在部署的時段（只換腳本、⛔ 不部署、⛔ 不碰服務）。
* 步驟（live 專人執行，指令另給，照 live 指令的格式：短行、base64＋sha256 核對）：
  1. 把新範本放到 scripts 目錄的暫存檔（例如 `deploy.sh.t084`），換掉五行，核對 SHA-256。
  2. **核對查的是正在用的那個 DB**（唯讀；填到另一個同樣有 `trading.backtest_jobs` 的 PostgreSQL 時，`--preflight` 照樣會回 `0|0`，只驗「找不到 container」驗不出這種錯）：
     * backend、python-server、python-worker 三個 container 的 `DATABASE_DSN` 只取出 host 與 database 名稱比對（⛔ 不印 DSN、⛔ 不印密碼）：
       host 要是 `DB_CONTAINER`（或它在同一個 docker 網路上的別名），database 要等於 `DB_NAME`；
     * 再從 python-server 用它自己的 DSN 唯讀查一次「`current_database()`、`pg_postmaster_start_time()`、`job_runs` 的最大 id」，
       跟用 `DB_CONTAINER`／`DB_USER`／`DB_NAME` 查到的同三個值比對；
     * 紀錄只留「三個服務一致／不一致」與比對的項目名稱，⛔ 不留 DSN、host 以外的連線資訊。不一致就停下、⛔ 不換檔。
  3. `./deploy.sh.t084 --preflight`：要通過，並印出 `pending 0、running 0`（有回測在跑時就是擋下，等它結束再試）。
  4. 另一份暫存檔把 `DB_CONTAINER` 改成不存在的名稱跑 `--preflight` → 要停下；刪掉這份。
  5. 備份 `deploy.sh` 成 `deploy.sh.bak-日期`，換上新版，記下 SHA-256。
* 換上之後的第一次部署，部署手冊 A 段的人工查詢照做，跟 `deploy.sh` 的結果對照一次。

## 版本紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-25 | 初版：使用者確認三項（查不到算失敗、⛔ 不留跳過的選項、DB 的 container 與帳號在開頭設定）；preflight 與 `down` 之前各查一次 |
| v1.1 | 2026-11-25 | 併入 v1 review：[1] 換檔前唯讀核對 `DB_CONTAINER`／`DB_NAME` 就是 backend 與兩個 python 服務正在用的 DB（DSN 只比 host 與 database，加上從 python-server 查的指紋比對），紀錄只留結果；[2] `docker inspect` 也包 `timeout -k`，補第二次查詢逾時⛔ 沒有 down 與強制終止的測試；[3] dev 的 SQL 實測固定用拋棄式 PostgreSQL（共用 dev DB 的 worker 會先認領）；成功終態更正為 `done` |
