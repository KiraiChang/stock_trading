# 舊 VPS 停機前一天的資料合併進新 live（I-131）——計畫書 v8

> ⛔ **不執行（2026-11-13）**：全表比對查明新 live 是用停機後的最終 dump 還原的，⛔ 沒有資料缺口，見 `docs/issue.md` I-131。本計畫書保留到 I-131 的 review 確認後刪除；作業隔離的做法已歸檔到 `docs/development-workflow.md`「在 dev 查 live 的真實資料：作業專用 volume」。
> 原狀態：v8 草稿，待使用者確認（2026-11-13）。v8 處理 v6、v7 的 review（R1～R5、P3）和 D5(d) 的整合評估；各版的 review 與修正見「十二、修訂紀錄」。依證據裁決策略的版本順延為 v9。
> ⛔ 確認前不建任何比對用的 DB、不對 live 下任何指令。對應：`docs/issue.md` I-131。

## 一、目標與不做的範圍

**目標**：新 live 的 DB 是用舊機 11-11 15:42 的 dump 還原的，但舊 live 一直寫到 11-12 約 16:00 才停。
要把這段只存在舊 DB 的資料補進新 live，同時不弄壞新 live 開始運作後寫入的資料。

**不做**：

* 不改 schema、API 或前端 contract，也不改產品程式。驗證工具例外（v8）：`scripts/smoke-dev.sh` 加上內部 health 模式和「不啟動 python-worker」的服務組合（6.1），新增 `scripts/check-isolation-config.py`（2.5），都要跟 v8 一起確認。
* 不處理 I-122～I-126 的觀察期判讀。那邊另外用封存的 log 與 dump 查，見 I-131「對其他項目的影響」。
* 不處理新機 live 本身的部署設定，例如重跑開關、「3.7」的確認。
* **策略 A／B 這一版不裁決**，等第五節的證據齊了再定（D1）。

## 二、角色與環境

### 2.1 分工（使用者 2026-11-13 定案）

* **live 由專人處理。** Claude ⛔ 不直接連 live，也不在 live 上執行任何指令。Claude 只寫出要執行的指令和預期輸出，由使用者交給專人執行，再把輸出貼回來。
* **取得 live 資料的方式（使用者 2026-11-13 定案）**：
  * **資料**：Claude 給 `pg_dump` 的指令，使用者在 live 上 dump，交給 dev 的受限目錄（`~/i131_merge/live/`，mode 700）。
    比對要用的查詢都在 dev 還原出來的 `merge_live`／`merge_live_frozen` 上由 Claude 跑，⛔ 不在 live 上跑 SQL 查詢。
  * **程式參數**（cron、開關、版本、對外入口、啟動 log 裡的註冊訊息）：Claude 給唯讀的 docker 指令，使用者在 live 上取出設定值後貼回來。
    指令只篩出需要的鍵，⛔ 不印其他環境變數，因為可能含密碼。
  * 指令和預期輸出見附錄 A。
* 萬一還是需要在 live 上跑查詢，一律唯讀：連線時帶 `PGOPTIONS='-c default_transaction_read_only=on'`，而且要先在 dev 的同 schema 資料上跑過。

### 2.2 比對與演練用的 DB：沿用 dev compose，⛔ 不另起 container

v1 原本要另外建 `st_merge_sandbox` container，這跟 repo 規定的「開發驗收只用 `docker-compose.dev.yml`」衝突，所以改成：

* 只啟動 dev compose 的 postgres，而且一定要疊上 2.5 的作業隔離 override（`OVR="$HOME/i131_merge/isolation.override.yml"`，然後 `docker compose -f docker-compose.dev.yml -f "$OVR" up -d postgres`）。postgres 掛的是作業專用的 volume，⛔ 不是原本的 dev volume（2.5）。
  它的 image 是 `postgres:16-alpine`，與兩份 dump 的 pg_dump 16.14 同一個大版本。資源限制照 dev 的設定（512m／0.5 CPU），⛔ 不另外調。
* 在作業 volume 的 postgres 裡建 `merge_` 開頭的 DB。原本 dev volume 裡的 `trading` DB 根本不會被掛載，所以⛔ 不會被動到；⛔ 不用 `down -v`。

| DB | 內容 |
|---|---|
| `merge_base` | 11-11 15:42 的 `trading.dump`：新 live 還原時的起點，也是三方比對的共同祖先 |
| `merge_old` | 11-12 16:05 停機後的最終 dump |
| `merge_live` | 新 live 的分析快照（4.3） |
| `merge_live_frozen` | 新 live 停寫後的快照（7.1）。執行前才會產生 |
| `merge_rehearsal_<n>` | 演練用，每次從上面幾個 DB 複製出來，用完就刪 |

* dump 用 `docker exec -i stock_trading_dev-postgres-1 pg_restore …` 從 stdin 餵進去，⛔ 不另外加掛載。
* 一律加上 `--no-owner --no-acl`。舊 dump 的擁有者是 `trading_username`，新 live 是 `trading_user`，dev 的角色是 `trading`（2026-11-13 比對 schema 查到）。比對時看的是資料，擁有者和權限⛔ 不在比對範圍內。
* 收尾：照 2.5 第 6 點，停掉作業服務、切回原本的 volume；作業 volume 依 D8 的時點整個刪掉（`merge_*` 也就一起消失）。

⚠️ **這些 DB 裝的是 live 的真實資料**，包括 `users` 的 email 和密碼雜湊、持倉和交易紀錄。dev 平常會發布 `127.0.0.1` 上的四個 port，帳密是 repo 裡寫的 `trading/trading`，所以真實資料⛔ 不能進原本的 dev volume。
怎麼保護見 2.5（D5）。

### 2.5 真實資料的保護（D5：建議採 (d)，v8 改成作業專用資料卷，待使用者確認）

**2026-11-13 在新機查到的事實**：

* Docker Engine 29.9.3、Compose v5.7.0。`development-workflow.md` 記的 Engine 20.10.5 是舊機的版本，⛔ 不能套用到新機。
* ✅ **dev 帳號用的是 rootless Docker**（v8 新增，2026-11-13 Claude 在 dev 查證）：`docker info` 的 security options 有 `rootless`，資料目錄在 `/home/dev/.local/share/docker`，context 是 `unix:///run/user/1001/docker.sock`；dev 帳號⛔ 不在 `docker` 群組裡。主機的 network namespace 只有 `lo`、`eth0` 和 rootful Docker 的 `docker0`，**看不到 dev 的 compose 網段**。rootless 模式下，container 的 bridge 在 RootlessKit 自己的 network namespace 裡，主機上的其他帳號只能經由**發布的 port** 連進來，⛔ 不能直接連 container IP。
* 主機上能登入的帳號（`getent passwd`，2026-11-13）：`root`、`kirai`（使用者本人）、`dev`（Claude 作業用）、`app`（live）。`app` 本來就擁有 live 的資料。
* `smoke-dev.sh` 的 health check 現在是從 host 打 `http://localhost:18080/health` 和 `:18001`，port 一撤就打不到（6.1 改成在腳本加上內部檢查的模式）。

**候選方案的評估**：

| 方案 | 評估 |
|---|---|
| (a) 臨時強密碼 | 當補強；只靠它不夠 |
| (b) 遮罩 email 和密碼雜湊 | ⛔ 不採用：會改變比對內容，不能當合併的資料來源；持倉、交易等敏感欄位也沒涵蓋 |
| (c) 限時接受風險 | ⛔ 不採用：只縮短暴露時間，沒有阻止存取 |
| **(d) 作業專用資料卷＋撤除所有 port＋臨時憑證＋全程保護** | **建議採用**（v8 依 v6 review R1 和整合評估調整）：真實資料只進作業專用的 volume，原本的 dev volume 從頭到尾⛔ 不掛載、也不會碰到 live 的資料，所以⛔ 不必在同一個 cluster 上「恢復公開帳密」，也⛔ 不必假設 `CHECKPOINT` 清掉了殘留的 WAL |

**(d) 的做法**：

1. **作業隔離 override**（放在 repo 外的受限目錄 `~/i131_merge/`，mode 600）：
   * 沿用同一份 `docker-compose.dev.yml`、同一個 project（`stock_trading_dev`），再疊上這個 override。⛔ 不另起 container。
   * **postgres 和 redis 改掛作業專用的具名 volume**：`stock_trading_dev_i131_pgdata`、`stock_trading_dev_i131_redisdata`。用 `volumes: !override` 整個取代原本的掛載清單，最終設定裡⛔ 不能出現原本的 `stock_trading_dev_postgres_data`、`stock_trading_dev_redis_data`。
     redis 也要換：smoke 會把演練 DB 的內容寫進快取，⛔ 不能帶回一般 dev 的 Redis。
   * **臨時隨機密碼**：作業 volume 是全新的，官方 postgres image 會在初始化時讀 override 裡的 `POSTGRES_PASSWORD`；redis 的 `REDIS_PASSWORD`／`REDISCLI_AUTH` 也換成臨時值。所以⛔ 不需要 `ALTER ROLE`。憑證只存在受限目錄的檔案裡，⛔ 不印到輸出，⛔ 不進版控。
   * **撤除所有 host port**：postgres、redis、backend、python-server、python-worker 都用 `ports: !reset []`。⛔ 不能假設 `ports: []` 就會移除原本的設定（`ports` 依 Compose 的合併規則是合併的）。
   * 2026-11-13 已在 dev 用 Compose v5.7.0 的 `config` 驗證這種寫法：五個服務的 port 都是 0 個；postgres、redis 都只掛作業 volume；原本的 dev volume 不在最終設定裡（只解析設定，⛔ 沒有啟動）。
2. **每次啟動前檢查最終設定（v6 review R3）**：
   * 用新的 repo 腳本 `scripts/check-isolation-config.py` 檢查：它從 stdin 讀 `docker compose … config --format json`，在記憶體裡解析，只輸出「服務名稱：通過／失敗」和失敗的檢查項目名稱。
   * 檢查項目：所有服務都沒有 port；postgres、redis 只掛指定的作業 volume；三個應用服務的 DSN 指向預期的 DB 名稱（只比對 DB 名稱）；**backend 的 `REDIS_PASSWORD` 跟 redis 服務的 `REDIS_PASSWORD` 相同、而且⛔ 不是 dev 的預設值 `dev-redis-only`**（只輸出相同／不同，⛔ 不印值）；postgres 的 `POSTGRES_PASSWORD` 也⛔ 不是 `trading`。
   * ⛔ 不印完整的 `config`、DSN 或憑證；解析失敗時也只印固定的錯誤訊息，⛔ 不回顯原始內容。
   * 這支腳本要先用合成的 JSON 測試（第九節）。
3. **啟動後的核對**：`docker ps` 的 port 欄、`ss -ltn` 確認 15432、16379、18080、18001 都沒有在監聽。
   **R5 的實測**：從主機的 network namespace（dev 帳號、⛔ 不進 RootlessKit 的 namespace）嘗試連作業 postgres 的 container IP 的 5432，必須連不上；結果記進演練紀錄。連得上的話就停下來，回到 D7 重新評估。
4. **核對角色與認證規則**：用 `\du` 和 `pg_hba_file_rules` 查作業 cluster 實際的角色與規則，記進演練紀錄。官方 image 的 `POSTGRES_USER` 是 superuser，container 內的本機連線不需要密碼（trust）；`docker exec psql` 只有 dev 帳號（和 root）做得到，屬於下面的保護邊界。
5. **全程保護**（v5 review R1）：
   * 從第一次匯入真實資料開始，到作業 volume 處置完畢為止，作業期間 dev 的 postgres／redis 一律帶著隔離 override 執行。**作業失敗、暫停或跨天時也一樣。**
   * ⚠️ 作業期間一般的 dev 驗收不能用（同一個 postgres container 掛的是作業 volume）。如果有人在作業期間不帶 override 跑了 dev compose（例如 `smoke-dev.sh`），postgres 會換回原本的 dev volume，**真實資料不會因此暴露**，但作業會被中斷，要重新帶 override 啟動並重做當下的步驟。
   * dump、差異明細、id 對照表、憑證都放在 `~/i131_merge/`（mode 700；`live/` 放 live 的 dump、`creds/` 放憑證、`out/` 放比對輸出）。
6. **結束時的順序**：
   1. 停掉作業中的應用服務，再停掉 postgres／redis（`docker compose … stop`，⛔ 不用 `down -v`：那會把原本的 dev volume 一起刪掉）；
   2. 拿掉隔離 override，照一般流程跑 `scripts/smoke-dev.sh`，確認 dev 回到原本的 volume、恢復正常（原本的 volume 從來沒有掛過真實資料，所以⛔ 不需要先清任何東西）；
   3. 作業 volume 先保留（保持未掛載），保留到 I-131 收斂、或使用者確認不再需要重跑為止；之後用 `docker volume rm` 指名刪除那兩個作業 volume。刪除的時點列入 D8，待 review 確認；
   4. 刪掉 scratchpad 裡含真實資料、而且⛔ 不需要保留的輸出。需要當證據保存的檔案（例如 id 對照表、各快照的 SHA-256），照原本的保存規則留在受限目錄。
   v7 原本的「`DROP DATABASE`、`CHECKPOINT` 之後把密碼改回 `trading`」整段刪除：原本的 cluster 從來沒有真實資料，不需要恢復；作業 cluster 不再恢復成公開帳密，直接整個刪掉。

**保護邊界**：這個做法防的是網路和一般 dev 存取。擁有主機 root、或能操作 dev 帳號 rootless Docker 的人（dev 帳號本身），屬於可信任的管理者範圍。live 的 `app` 帳號本來就擁有這份資料。

### 2.3 排程器沒有整體開關（2026-11-13 讀程式碼確認）

* backend 啟動時無條件呼叫排程器的 `Start()`，⛔ 沒有任何設定可以讓 backend 跑起來、排程器卻不跑。
* `Start()` 裡**無條件註冊**的工作有：`pre_market`（`50 8 * * 1-5`）、`intraday`（`*/5 9-13 * * 1-5`）、`daily_close`（`0 15 * * 1-5`，`sr_zone_verify` 跟著它跑），以及 `chip_daily_sync`（cron 可以設定，但⛔ 沒有 enabled 開關）。
* `corporate_action_sync` 只要公司行動的元件存在就會註冊，⛔ 沒有 enabled 開關。cron 預設是 `30 6 * * 1-5`。
* 有 enabled 開關的工作：`stock_symbol_sync`、`evaluation_universe_sync`、日 K 缺漏偵測、`sr_analysis`、`sr_analysis_chip`、`delisting_reconcile`、`sr_evaluation`。
* 層一的重跑是記憶體裡的計時器（`time.AfterFunc`），backend 一停就消失，⛔ 不會在重啟後自己接續。
* python 端⛔ 沒有自己的排程器：`python/` 裡沒有 apscheduler 這類套件。
* python-worker 是**輪詢 DB** 的：每隔 `WORKER_POLL_INTERVAL` 秒掃描 `backtest_jobs` 裡 `status='pending'` 的任務並執行，執行結果會寫進回測相關的表。v4 寫成「只消化 Redis 佇列」，那是錯的。

所以「把排程開關都關掉」⛔ 做不到排程全停。dev 的 smoke（6.1）和 live 的封鎖（7.3）都改用**時間窗隔離，再加上事後驗證**。
如果要真正的整體開關，就得改程式。那不在本案範圍內，需要另外立案。

### 2.4 backend 啟動時本來就會寫 DB（2026-11-13 讀程式碼確認）

即使沒有任何排程觸發，backend 一啟動就會寫入下面這些。dev 的 smoke（6.1）和 live 的封鎖期驗收（7.3）都要把它們列成**預期的變化**：

| 來源 | 會改什麼 | 預期的變化 |
|---|---|---|
| 自動套用 migration | `goose_db_version` 和 schema | 兩邊的 schema 版本相同，應該**沒有變化**。啟動前後要核對 `goose_db_version` 的最大版本 |
| 回收孤兒紀錄（`AbortRunning`，在排程器啟動之前執行） | `job_runs` 裡啟動前所有 `status='running'` 的列 | 只有這些列會變，而且只改三個欄位：`status` 改成 `aborted`、`error` 改成程式裡固定的回收訊息、`finished_at` 改成啟動當下的時間 |
| 取得 JWT 時註冊驗收帳號（smoke 需要登入時才有） | `users`，以及註冊時順帶建立的其他列 | 註冊會一起建立哪些列，smoke 之前要先讀程式碼列出來；能不用登入就不要用 |

驗證方式：啟動前先記下 `status='running'` 的 id 集合 R，以及逐表的內容雜湊。啟動之後：

* R 裡的每一列只能有上面那三個欄位改變，而且是預期的值；
* `job_runs` 裡不屬於 R 的列，雜湊⛔ 不能變；
* 註冊驗收帳號新增的列要跟事先列出的一致；
* 其他表的雜湊都⛔ 不能變。

只要有任何不在預期清單裡的變化，就判失敗。

## 三、步驟 1：建 DB、還原兩份舊 dump（只在 dev，不需要 live）

1. 照 2.5 完成作業隔離（撤 port、檢查最終設定、臨時密碼、核對角色與認證規則），**之後**才啟動 dev postgres，還原 `merge_base` 與 `merge_old`。
   兩份 dump 的 schema 已確認相同：51 張表，時間欄位都是 `timestamptz`（2026-11-13 用 `pg_restore --schema-only` 比對過）。
2. 逐表以 PK 對齊，算出 `merge_base → merge_old` 的新增、更新、刪除。
   ⚠️ **這只是「舊機這段期間改了什麼」，⛔ 不等於新 live 實際缺了什麼。** 實際缺漏要等第四節的三方比對才能確定。
3. 把要給專人跑的唯讀查詢（4.1）先在 `merge_old` 上跑過。

## 四、步驟 2：新 live 的快照與三方比對

### 4.1 新 live 的查詢（在 dev 的 `merge_live` 上跑，要先完成 4.3）

查詢先在 `merge_old` 上驗證過，再對 `merge_live` 跑。⛔ 不在 live 上跑。

* 每張表的總筆數、`max(id)`，以及時間欄位 ≥ 2026-11-11 15:42 +08:00 的筆數。
  時間欄位依序取 `created_at`、`started_at`、`fetched_at`、`analyzed_at`、`added_at`、`occurred_at`、`last_attempted_at`；有 `updated_at` 的表，另外算更新的筆數。
* `goose_db_version` 的最大版本：確認新 live 沒有跑過新的 migration。
* 每個 sequence 的 `last_value`。
* 人工輸入的表，從 11-11 15:42 起有沒有新增或更新：`users`、`tenants`、`tenant_members`、`portfolio_groups`、`group_members`、`portfolios`、
  `position_transactions`、`positions`、`watchlists`、`backtest_*`、`sr_scoring_train_jobs`、`market_backfill_jobs`。

### 4.2 要請專人記錄的時間

* 新 live 用 dump 還原的時間：DB 裡沒有紀錄，請使用者依當時的操作紀錄回報。
* 第一次開始寫入的時間：由 Claude 從 `merge_live` 推算（11-11 15:42 之後最早的時間欄位），再請使用者核對。I-131 目前記為未記錄。

### 4.3 分析快照

使用者照附錄 A-1，用 `pg_dump -Fc` 做一份新 live 的快照，記下 SHA-256，交到 `~/i131_merge/live/`。等 2.5 的隔離完成之後，才還原成 `merge_live`。
這份快照是在 live **還在運作時**做的，只拿來分析和演練；執行時要的是停寫後的快照（7.1）。
A-1 的指令會印出 `pg_dump` 的開始時間、結束時間和耗時；交接到 dev 花多久也請記下來。這兩個數字要拿來估 8.2 的 t_freeze 和 t_handoff。

### 4.4 三方比對（`merge_base` 是共同祖先）

比對分兩層。第一層是互斥的分類，第二層是額外的檢查。⚠️ v2 把兩者混成一張表，結果「每列只屬一類」不成立：漏了未變更的列，撞號又同時屬於兩類。

**第一層：以 PK 做互斥分類**

* 範圍是三個 DB 的 PK 聯集，每一張表各自算。
* 每個 PK 在每一邊的狀態只有兩種：
  * `⊥`：這一邊沒有這個 PK；
  * 該列的內容雜湊：所有欄位依欄位順序轉成文字後取雜湊，NULL 用固定的標記表示。
* **刪除**的表示：base 有這個 PK，這一邊是 `⊥`。**新增**的表示：base 是 `⊥`，這一邊有。這兩種情況都跟「內容改了」一樣，算成「這一邊跟 base 不同」。
* 依下表分類。每個 PK 只會落在其中一類：

| 分類 | 條件（b＝base、o＝old、l＝live） | 預設處置（待 D1／D3 確認） |
|---|---|---|
| U 未變更 | o = b 且 l = b | 不用處理 |
| O 只有舊機改 | o ≠ b 且 l = b | 把舊機的改動補進 live：新增、更新或刪除 |
| L 只有新 live 改 | o = b 且 l ≠ b | 保留 live 的 |
| S 兩邊改成一樣 | o ≠ b、l ≠ b、o = l | 不用處理（兩邊都刪掉也算這一類） |
| C 兩邊改得不同 | o ≠ b、l ≠ b、o ≠ l | **衝突：逐表訂仲裁規則**（D3）。包括「一邊刪、一邊改」，以及「兩邊各自新增了同一個 PK」 |

* 驗收條件：U＋O＋L＋S＋C 的筆數要等於 PK 聯集的筆數。報告要逐表列出五類的筆數，C 的每一筆都要列出來。

**第二層：額外的檢查（標記，⛔ 不是分類，可以跨類）**

* **id 撞號**：第一層 C 類裡，b = `⊥`、o 和 l 都有、內容不同、而且這張表有 sequence 的列。
  它代表兩邊各自取到同一個號碼，⛔ 不是同一筆資料被兩邊改。要換 id（4.5），⛔ 不能當成一般衝突來仲裁。
* **自然鍵重複**：有自然鍵（unique 約束）的表，先把 O 類和 C 類照預設處置模擬合併，再找「同一個自然鍵對到兩個以上 PK」的組合。
  這種組合可能橫跨 O、L、C 三類。每一組都要列出來，依 D3 決定留哪一邊。
* **參照懸空**：模擬合併後，FK 和隱含參照（E5）指向不存在的列。

### 4.5 id 與參照的對照（如果要換 id）

新 live 的 sequence 是從 11-11 15:42 的狀態接著用的，所以兩邊這段期間新增的列 id 會重疊。
不論 A 或 B，**搬過去的那一邊**都要換新 id，參照到它的子表也要跟著改。
對照表（`表, 舊 id, 新 id`）存在 repo 外的工作目錄，並記下 SHA-256。

schema 裡的 FK（2026-11-13 從 dump 整理）：

| 父表 | 參照它的子表．欄位 |
|---|---|
| `job_runs` | `delisting_source_snapshots.job_run_id`（`ON DELETE SET NULL`） |
| `stock_sr_zone_analyses` | `market_event_detections`、`market_event_states`、`stock_sr_daily_candidates`、`stock_sr_decisions`、`stock_sr_model_governance`、`stock_sr_zones` 的 `analysis_id` |
| `stock_analyses` | `stock_analysis_levels.analysis_id` |
| `backtest_jobs` | `backtest_results.job_id`、`backtest_trades.job_id` |
| `sr_scoring_train_jobs` | `stock_sr_model_metrics.train_job_id` |
| `delisting_events` | `stock_symbols.delisted_event_id`（`ON DELETE RESTRICT`） |
| `portfolios` | `position_analyses`、`position_transactions`、`positions` 的 `portfolio_id` |
| `position_transactions` | `positions.last_event_id` |
| `users`／`tenants`／`portfolio_groups` | `tenant_members`、`group_members`、`portfolios`、`portfolio_groups` |
| `zone_instances` | `event_instances`、`zone_key_aliases`、`zone_relations`（parent／child）、`zone_role_incarnations`、`zone_transitions` 的 `zone_uid` |
| `zone_role_incarnations` | `zone_transitions.incarnation_uid` |
| `event_instances` | `event_transitions.event_uid` |

⚠️ **不是 FK 的隱含參照還沒盤點**：例如 JSON 欄位裡的 id、以 `job_id` 字串對應的紀錄、log 和文件裡引用的 `job_runs` id。
要讀程式碼確認（E5）。另外，`zone_uid`、`event_uid`、`incarnation_uid` 是文字 PK，產生方式要確認（E3）：
* 如果是確定性的（由身分推導），兩邊會用同一個 uid 代表同一個 zone，狀態卻不同 → 狀態機衝突。
* 如果是隨機的，同一個 zone 會出現兩筆 → 身分重複。

## 五、仲裁：策略 A／B 都保留，依證據裁決（D1）

### 5.1 兩種策略

**A：把舊機的差異合併進目前的新 live**

* 在 live 上執行。只碰 4.4 分類出來的列。
* 「只有舊機改」的新增列換新 id 之後插入（4.5）。
* 衝突依 D3 處理。

**B：以最終 dump 為基準重建，再把新 live 的寫入搬回來**

⚠️ 如果採 B，在 live 上還原最終 dump 時，要把擁有者對到新 live 的 `trading_user`，也要保留新 live 收緊過的 `public` 權限（`REVOKE USAGE … FROM PUBLIC`）。⛔ 不能直接沿用舊 dump 的擁有者和權限。

* 以 `merge_old` 的內容重建 live 的 DB。舊機的 id 都不變；新 live 寫入的列換新 id 之後搬回來。
* 衍生結果（SR 分析、訊號、zone／event 生命週期、指標快照）可以搬回，也可以改用重跑補齊，看 E4。

### 5.2 裁決需要的證據

| 編號 | 證據 | 怎麼取得 | 影響 |
|---|---|---|---|
| E1 | 三方比對的逐表筆數（4.4） | 第三、四節 | 兩種策略各要搬多少、要仲裁多少 |
| E2 | 兩邊各有多少人工輸入的資料 | 4.1、4.4 | 人工資料必須逐筆確認，⛔ 不能靠重跑 |
| E3 | `zone_uid`／`event_uid`／`incarnation_uid` 的產生方式 | 讀程式碼 | 狀態機表能不能逐列合併 |
| E4 | **歷史 SR 和狀態機能不能對過去的日期補跑**，而且結果跟當天正常排程跑出來的一樣 | 在 dev 實測：用 `merge_base` 的複本補跑 11-11 之後的批次，跟 `merge_old` 比對 | ⚠️ **B 是否可行的前提；v1 誤當成已知** |
| E5 | 不是 FK 的隱含參照 | 讀程式碼 | 換 id 的範圍 |
| E6 | 原始市場資料能不能重抓：例如盤中 K 棒、籌碼的歷史可以回補多遠 | 讀程式碼與資料源文件 | 衍生結果能不能用重跑取代搬回 |
| E7 | live 的對外入口（反向代理、API、手動觸發端點能不能從外部打到），以及**每一個已註冊工作的 cron 和開關的實際值**，包括 2.3 列的固定註冊和沒有開關的工作 | 使用者照附錄 A-2 從 live 的 docker 取出設定值；Claude 對照 2.3 核對 | 7.3 的封鎖方式；時間窗隔離能不能成立 |
| E8 | 錯過的 cron 會不會補跑；手動補跑的入口和限制 | 讀程式碼 | 8.3 的補跑方式 |
| E9 | `evaluation_universe_sync`、`sr_analysis` 平常要跑多久 | 從 `merge_live` 和舊 dump 的 `job_runs` 統計 | 8.2 的 T0，以及傍晚窗口實際可用的時間 |

### 5.3 狀態機類的表（不論 A 或 B 都要處理）

`zone_instances`、`zone_key_aliases`、`zone_relations`、`zone_role_incarnations`、`zone_transitions`、`event_instances`、`event_transitions`、
`positions`（它是 `position_transactions` 的投影）、`candle_verification_state`、`stock_symbols` 和 `delisting_events`（上市狀態）、`evaluation_universe`。

* 兩邊都從 11-11 的狀態各自往前推進過。⛔ **不能把兩邊的列混在一起**，否則狀態的前後順序會矛盾。
* 候選做法（依 E3、E4 選）：
  * (a) 每個身分（例如每檔股票）整組只取一邊，再補跑缺的日期；
  * (b) 整張表只取一邊，再補跑缺的日期；
  * (c) 如果 E4 證明可以補跑，就從共同祖先重建。
* `positions` 一律由 `position_transactions` 重新投影。⛔ 不合併投影結果。

### 5.4 其他要決定的事

* **D2**：`job_runs` 換 id 時要不要標記來源。⚠️ 欄位語意屬於 contract，**傾向只記對照表、不改欄位內容**。
* **D3**：衝突與自然鍵重複的仲裁規則。逐表定，寫成一張表附在 v8。
* **D4**：缺的批次怎麼補（排程或手動入口），以及補的順序（日 K → 指標 → `sr_analysis` → 籌碼兩輪）。
* **D5**：`merge_*` 裝的是 live 的真實資料，暴露風險怎麼處理。**建議採 (d)，v8 改成：作業專用資料卷＋撤除所有 port＋臨時憑證＋全程保護**，(a) 當作其中的一部分，(b)、(c) 不採用。評估和做法見 2.5，待使用者確認。
* **D7**（v8 新增，對應 v7 整合評估的 R5）：如果 2.5 第 3 點的實測發現，從主機的 network namespace 連得到作業 container 的 IP（也就是 rootless 的隔離不成立），要怎麼處理。**建議**：停下來，⛔ 不匯入真實資料；改由專人規劃只限作業網段的存取限制並實測，⛔ 不動 live 的網路或全域防火牆。依 2026-11-13 查到的事實（2.5），預期不會走到這一步。
* **D8**（v8 新增）：作業 volume 什麼時候刪。**建議**：I-131 收斂、或使用者確認不再重跑演練之後，用 `docker volume rm` 指名刪除；在那之前保持未掛載，⛔ 不用 `down -v`。

## 六、步驟 3：在 dev 演練

1. 把選定的策略寫成腳本，對 `merge_rehearsal_<n>` 執行。A 的起點是 `merge_live` 的複本，B 的起點是 `merge_old` 的複本。
2. 驗證項目：
   * 所有 PK、unique、FK 都成立。
   * 逐表筆數 = 起點筆數 + 應搬入筆數 − 應覆蓋或刪除筆數。以 4.4 的報告為準，逐表列出。
   * 每個 sequence 的 `last_value` 都 ≥ 該表的 `max(id)`。
   * 4.1 和 4.4 列出的每一筆人工輸入資料都在結果裡，而且內容一致。
   * 狀態機表照 5.3 選定的做法處理之後，每個身分的狀態都只來自一邊，或來自重建。
   * `positions` 重新投影的結果跟 `position_transactions` 一致。
   * 讓 dev 的服務連到演練用的 DB，做 smoke（見 6.1）。⛔ 不用 live。
3. 可重跑：同一個起點跑兩次，結果（逐表的內容雜湊）要相同。
4. 記錄時間，用來排第八節的時間表（代號見 8.2）：
   * t_script：腳本跑多久；
   * t_restore：用停寫快照大小的 dump，在 dev 還原一次要多久，也就是回滾時間的估計值；
   * t_restore_dev：停寫快照還原成 `merge_live_frozen` 要多久，t_compare：它跟 `merge_live` 比對要多久；
   * t_accept：第七節的驗證查詢加上啟動確認要多久；
   * t_postsnap：演練結果做一次 `pg_dump` 要多久，這是合併後快照的估計值；
   * t_freeze 和 t_handoff 用 4.3 專人記下的數字。

   dev 的資源限制（512m／0.5 CPU）比 live 緊，量到的時間可以當保守估計，⛔ 但不能當上限的保證。

### 6.1 讓 dev 的服務連到演練用的 DB

⚠️ dev compose 的 backend、python-server、python-worker 的 `DATABASE_DSN` 都寫死連到 `trading`。另外，2.5 撤掉了 host port，`smoke-dev.sh` 現在的 health check 打的是 host port。
v8 依 v6 review R2 改成：**驗收照 repo 的「測試腳本優先」規範，用 `scripts/smoke-dev.sh` 執行**，先把需要的能力補進腳本（這是驗證流程的修改，屬於本計畫的範圍，要跟 v8 一起確認）。v7 的「直接下 compose 指令、⛔ 不修改 `smoke-dev.sh`」作廢；一次性的指令只能拿來診斷。

**`smoke-dev.sh` 要補的能力**（預設行為⛔ 不變）：

* `HEALTH_MODE=internal`：health check 改從 container 內部打。backend 用 `compose exec -T backend wget -qO- http://localhost:8080/health`（image 是 alpine，有 busybox 的 `wget`）；python-server 用 container 裡的 `python` 打 `http://localhost:8001/health`。逾時、失敗診斷、`show_diagnostics` 照舊。預設值 `host`，行為跟現在相同。
* 明確指定 `SERVICES=backend,python-server` 時，改用部分服務模式：先用 `--wait` 把 postgres、redis 起到 healthy，再用 `--no-deps` 啟動這兩個服務，**⛔ 不啟動 python-worker**。現在的寫法是兩個都選就進 `all` 模式，會把 python-worker 一起起來（python-worker 會執行 pending 的回測任務，2.3）。`SERVICES` 沒設定時的預設行為不變。
* 既有的保護全部保留：`COMPOSE_PROJECT_NAME` 有設定就中止、`OVERRIDE` 只接受一個檔、依賴服務的 readiness、逾時。
* 腳本修改的驗證：在 dev（⛔ 沒有真實資料的原本 volume）上跑三種組合：預設、`HEALTH_MODE=internal`、`SERVICES=backend,python-server`，確認行為符合預期，而且第三種⛔ 沒有啟動 python-worker。

**步驟**：

1. **override 檔**：用 2.5 的隔離 override（只有這一個檔，放在受限目錄、mode 600），在裡面多寫幾項：
   * backend 和 python-server 的 `DATABASE_DSN` 都改成 `merge_rehearsal_<n>`，帶上臨時密碼；
   * **backend 的 `REDIS_PASSWORD` 改成跟作業 redis 相同的臨時密碼**（v8 review）：dev compose 的 backend 是 `${DEV_REDIS_PASSWORD:-dev-redis-only}`，只換 redis 那一側會變成 `WRONGPASS`。現在的 backend 會靜默停用 Redis、health 照樣通過，smoke 驗不到快取；I-132 的 C 上線之後則會直接起不來。兩邊的值放在同一個憑證檔，override 從那裡寫入，⛔ 不手打兩次；
   * python-worker 的 DSN 也改成同一個 DB，以防它被意外啟動。⛔ 不能留著原本的 DSN；
   * 排程的設定（第 2 點）。
2. **排程的時間窗隔離**：關開關只能關掉有開關的那幾個工作（2.3）。所以：
   * 同一個 override 檔把有開關的工作都關掉，沒有開關但 cron 可以設定的（`chip_daily_sync`、`corporate_action_sync`）把 cron 改到 smoke 的時段之外；
   * smoke 選在固定註冊的工作都不會觸發的時段：週末，或平日 15:30 之後到隔天 06:30 之前；
   * **事後驗證**：照 2.4 的方式，smoke 前記下 R 和逐表內容雜湊，smoke 後逐項核對。`job_runs` 只要有新增，或出現任何不在預期清單裡的變化，這次 smoke 就作廢，查明原因後重做。這段核對用第六節第 1 點的演練腳本執行，⛔ 不用一次性指令。
3. **啟動前**：用 `scripts/check-isolation-config.py` 檢查最終設定（2.5 第 2 點），DSN 的檢查項目指定 `merge_rehearsal_<n>`。image 必須是現行的程式碼；需要重新 build 的話，在匯入真實資料之前先照一般流程 build 好。
4. **啟動與 health**：先設 `OVR="$HOME/i131_merge/isolation.override.yml"`，再執行 `NO_BUILD=1 SKIP_DOWN=1 HEALTH_MODE=internal SERVICES=backend,python-server OVERRIDE="$OVR" scripts/smoke-dev.sh`。
   `SKIP_DOWN=1`：作業期間⛔ 不讓腳本先 `down`，以免中斷正在作業的 postgres。
5. **核對實際連到哪個 DB 和 Redis**：在作業 postgres 查 `pg_stat_activity`，依 `datname` 和 `application_name` 分組計數，應用程式的連線都要在 `merge_rehearsal_<n>`。backend 的 log 要有 `redis connected`，⛔ 不能有 `redis connect failed`。⛔ 不在 container 裡印環境變數。
6. **功能確認**：從 container 內部打 `/scheduler/status` 和主要的讀取 API，結果要跟演練 DB 的內容一致（例如持倉筆數）。這一段寫進演練腳本，跟第 2 點的事後驗證放在一起。
7. **smoke 之後**：
   * 停掉 backend 和 python-server。
   * ⛔ 這裡不拿掉隔離 override：作業 volume 還在用，保護要一直維持到 2.5 第 6 點的結束順序（v5 review R1）。
   * 下一輪演練就換新的 `merge_rehearsal_<n>`，DSN 改成新的名稱。

## 七、步驟 4：live 執行（專人執行）

### 7.1 停寫快照與執行前的關卡

1. 專人停掉新 live 的 backend 和 python 服務（排程跟著停），確認已經沒有寫入。
2. 照附錄 A-1 的同一套指令（檔名前綴改成 `live_frozen_`）做停寫快照，記下 SHA-256。**這份同時是回滾用的備份**，live 那邊的原檔也要保留。
3. 把停寫快照交給 dev，還原成 `merge_live_frozen`，跟演練時用的 `merge_live` 比對（逐表筆數、`max(id)`、內容雜湊）：
   * 如果相同：可以直接用演練過的腳本。
   * 如果不同（分析快照之後又有寫入）：⛔ 不能直接執行。要用 `merge_live_frozen` 重做 4.4 和第六節，並重新取得使用者確認。這段期間 live 要不要先恢復服務，由使用者決定。

### 7.2 執行

依策略，交給專人逐步執行的指令（附預期輸出），每一步的輸出都貼回來核對之後才進下一步。

### 7.3 封鎖期間的驗收

⚠️ v2 的寫法是先啟動服務、再做確認和補跑，最後才決定要不要回滾。這樣一回滾，恢復服務之後的新寫入（例如人工交易）就會一起消失。v3 改成：
**驗收全部做完之前不開放任何外部寫入。開放之後，就⛔ 不再用停寫快照回滾。**

1. **封鎖的範圍**：從 7.1 停服務開始，直到 7.4 解除封鎖為止。以下都不能發生：
   * 外部使用者透過 API 或前端寫入（交易、持倉、自選股、回測、手動觸發工作）；
   * 排程觸發任何工作；
   * python-worker 執行 `backtest_jobs` 裡 pending 的回測任務（2.3）。
2. **怎麼封鎖**（具體指令要等 E7 的回報）：
   * **對外入口**：由專人讓反向代理（caddy）對外回維護頁，或關閉對外的路由。內部的 smoke 只用讀取的 API，⛔ 不打手動觸發的端點。
   * **python-worker**：封鎖期間⛔ 不啟動。
   * **排程：用時間窗隔離，⛔ 不靠開關**（2.3）。backend 一啟動，所有已註冊工作的「下一次觸發時刻」都必須晚於 T_open。
     依 D6 選定的平日傍晚窗口（8.2）：固定註冊的 `pre_market`（08:50）、`intraday`（09:00～13:55）、`daily_close`（15:00）在 17:00 之後到隔天 08:50 都不會觸發；
     cron 可以設定的工作，用附錄 B 的 live 實際設定逐一核對。17:00 之後第一個觸發的是沒有開關的 `chip_daily_sync`（22:00），所以 T_iso＝21:55。
     只要有任何一個工作的觸發時刻落在封鎖期間，就⛔ 不能在那個時段執行，改選時段。
   * **隔離截止時刻 T_iso**＝backend 啟動之後，所有已註冊工作中最早的那一個觸發時刻，再減 5 分鐘（用 E7 的實際設定計算；D6 的平日傍晚窗口是 22:00 − 5 分鐘＝21:55）。
     **到了 T_iso 還沒解除封鎖，就一定要停掉 backend**，不管當時進行到哪一步。這樣就不會有工作在封鎖期間觸發。
     T_open（21:45）一定要早於 T_iso；算出來不成立的話，那個時段就⛔ 不能執行。
   * **事後驗證**：一共兩次。第一次在合併後快照開始之前（第 3 點第 4 步），第二次在快照完成之後、解除封鎖之前。
     兩次都照 2.4 核對：從 backend 啟動到當下，`job_runs` ⛔ 不能有新的列，只能有 2.4 列出的預期變化。
     第一次不通過，視為驗收不通過（第 3 點第 3 步）。第二次不通過，代表快照期間有工作觸發，這時要停掉 backend、保持封鎖，交給使用者決定怎麼處理。
3. **封鎖期間的驗收順序**：
   1. 只有 postgres 在跑：跑演練時同一組驗證查詢，結果要跟演練一致。同時記下 2.4 要用的 R 和逐表內容雜湊。
   2. 在封鎖狀態下啟動 backend 和 python-server（對外入口仍然封鎖，python-worker 不啟動），跑部署後的確認（含 Redis），再從內部打讀取的 API 做 smoke。啟動造成的寫入照 2.4 核對。
   3. 任何一項不符，或第 2 點的事後驗證不通過 → 回滾（8.4）。這時還沒有外部寫入，回滾⛔ 不會丟資料。
   4. 全部通過之後，**仍在封鎖中**，做**合併後快照**（`pg_dump -Fc`，記下 SHA-256）。它是 7.4 第 3 點找出「解除封鎖之後的寫入」的共同祖先。⛔ 沒有這份快照就不准解除封鎖。
      快照只需要 postgres。如果快照進行中到了 T_iso，照第 2 點停掉 backend，快照照樣做完。
4. **缺的批次（D4）**：封鎖期間⛔ 不補跑。批次會寫入大量衍生資料，而且耗時，會壓縮回滾時間。等 7.4 解除封鎖之後再補。

### 7.4 解除封鎖與之後

1. 解除封鎖的條件：驗收全部通過、合併後快照已完成、第二次事後驗證也通過。解除封鎖有兩條路徑，⛔ 沒有第三條：
   * **正常路徑**（T_open 之前）：backend 還在跑。先啟動 python-worker，再開放對外入口。排程在 backend 啟動時就已經註冊了，所以⛔ 不需要另外「開啟排程」。
   * **超時路徑**（8.2 檢查點 ③ 不成立，或已經過了 T_open）：到了 T_iso 時 backend 已經照 7.3 第 2 點停掉。快照和第二次事後驗證都完成之後，先過**重啟關卡**（v8 review），通過才重新啟動 backend；重新啟動時一樣要照 2.4 核對 `AbortRunning`。接著立刻啟動 python-worker、開放對外入口，然後依 8.3 補跑被錯過的工作。
     * **重啟關卡**：用**當下的時間**和 E7 的設定，重新算出每個已註冊工作的下一次觸發時刻，取最早的一個 t_next。必須符合「現在 + t_reopen + 5 分鐘 < t_next」才可以重啟。t_reopen 是 backend 啟動加上 python-worker 啟動和開放入口的時間，用演練實測的值（第六節第 4 點）。
     * 不符合時，backend **繼續保持停止**，等 t_next 過了之後再重算一次關卡（被這樣跳過的工作，加進 8.3 的補跑清單）。⛔ 不能在入口還封鎖的時候，讓排程在 backend 裡觸發。例如 21:58 才完成快照，22:00 的工作就要先跳過，22:00 之後重算。
     * 每一次的判斷（當下時間、t_next、t_reopen、結果）都記進執行紀錄。

   開放的時刻要記下來。
2. 解除之後，依 D4 補跑缺的批次。補跑失敗就修正後重跑（批次本身要可以重跑），⛔ 不回滾 DB。
3. **解除之後如果發現資料錯誤**：⛔ 不能直接還原停寫快照，因為會丟掉解除之後的寫入。改成：
   1. 先做一份當下的 dump（記 SHA-256）保全資料；
   2. 以 7.3 第 3 點第 4 步的**合併後快照**當共同祖先，用第四節的三方比對找出解除之後的寫入。⛔ 不能用停寫快照，因為會把本次合併造成的變更也算進去；
   3. 修正方案另外寫進本計畫書的修訂、經使用者確認後才執行。
   解除之後要不要再封鎖、封鎖多久，由使用者決定。

## 八、交易語意、時間表與回滾

### 8.1 哪些能靠 transaction，哪些不能

| 操作 | 能不能 rollback | 處置 |
|---|---|---|
| A 的資料寫入（INSERT／UPDATE／DELETE） | ✅ 在單一 transaction 內，失敗就整筆 rollback | 腳本一律用 `ON_ERROR_STOP`，包在一個 transaction 裡 |
| `nextval`（插入時取號） | ⛔ 非交易性：rollback 後號碼不會還回去 | 只會留下 id 空號，⛔ 不影響正確性；⛔ 不依賴 id 連號 |
| `setval`（調整 sequence） | ⛔ 非交易性：rollback 後不會還原 | 只能往上調、⛔ 不往下調；放在資料 commit 之後另一步執行，執行前後的值都要記下來 |
| B 的整庫重建（`DROP／CREATE DATABASE` 加上 `pg_restore`） | ⛔ `DROP／CREATE DATABASE` 不能放進 transaction；`pg_restore --single-transaction` 只保護還原的內容 | 先還原到**新的 DB 名稱**，驗證通過之後才切換；舊的 DB 留到回滾期限過了才刪 |
| 補跑批次 | ⛔ 每個批次各自 commit | 批次本身要可以重跑；失敗就看 `job_runs` 個別重跑 |

### 8.2 時間表

⚠️ v2 把回滾期限訂在隔天 08:00，晚於 06:30 的早班批次。v3 起改成依實測時間倒推。v7 依 D6（使用者 2026-11-13 定案）改成**平日傍晚的窗口**：

| 時刻 | 規則 |
|---|---|
| 最早可以開始停寫（T0） | 當天 16:00 的 `evaluation_universe_sync`（含日 K 缺漏偵測）和 17:00 的 `sr_analysis`，在 `job_runs` 都顯示已結束之後。⛔ 不能在 17:00 之前就開始封鎖，否則 `sr_analysis` 會在封鎖期間觸發。`sr_analysis` 一般要跑多久，從 `merge_live` 的 `job_runs` 查（E9）。⚠️ T0 只是下限，⛔ 不代表實際進度；實際能不能繼續，看下面三個檢查點 |
| 最晚解除封鎖（T_open） | **21:45**：比 22:00 的 `chip_daily_sync` 早 15 分鐘，讓排程在解除之後確實恢復運作。這是**正常路徑**的期限 |
| 隔離截止（T_iso） | 7.3 第 2 點：最早的觸發時刻 − 5 分鐘。平日傍晚窗口是 **21:55**（`chip_daily_sync` 22:00；附錄 B）。backend 在封鎖中只能跑到這個時刻；T_open 必須早於 T_iso |
| 最晚開始回滾（T_rb） | T_open − 回滾所需時間 − 30 分鐘餘量。回滾所需時間＝還原停寫快照＋第七節的驗證查詢＋啟動確認，用第六節第 4 點的實測值，取保守的那一邊 |

**三個檢查點**：每一個都用**當下的時間**重新判斷，⛔ 不沿用前面算過的結果。

| 檢查點 | 時機 | 條件 | 不成立時 |
|---|---|---|---|
| ① | 7.1 停服務之前 | 現在 ＋ t_freeze ＋ t_handoff ＋ t_restore_dev ＋ t_compare ＋ t_script ＋ t_accept ≤ T_rb，而且再加上 t_postsnap ＋ t_unlock 之後 ≤ T_open | ⛔ 不停服務，改天再做 |
| ② | 7.1 的核對完成之後、7.2 執行之前 | 現在 ＋ t_script ＋ t_accept ≤ T_rb，而且再加上 t_postsnap ＋ t_unlock 之後 ≤ T_open | ⛔ 不執行 7.2。live 的資料還沒動過，所以直接恢復服務就好（啟動 backend 和 python-worker、開放入口），這⛔ 不算回滾；改天再做，到時候要重做停寫快照 |
| ③ | 7.3 驗收通過之後 | 現在 ＋ t_postsnap ＋ t_unlock ≤ T_open | 改走 7.4 第 1 點的**超時路徑**：合併後快照照樣要做，⛔ 不能省；到了 T_iso 就停掉 backend，所以 22:00 之後的工作⛔ 不會在封鎖期間觸發，而是被錯過；快照和第二次事後驗證都完成後重新啟動，再依 8.3 補跑 |

t_unlock 是解除封鎖的步驟要花的時間（啟動 python-worker、開放入口），由專人估計。其餘代號見第六節第 4 點。

* 到了 T_rb 驗收還沒全部通過，就直接開始回滾，⛔ 不再延長驗收。
* 實際的數字在演練之後填進 v8，執行前要經使用者確認。

**執行時段（D6，使用者 2026-11-13 定案：平日 17:00 之後）**：

| 候選 | 下一個觸發 | 可用時間 | 結論 |
|---|---|---|---|
| 平日 14:00 | `daily_close` 15:00 → T_iso 14:55、T_open 14:45 | 不到 1 小時；T_rb 大約 14:05 | ⛔ 不採用：停寫、交接、還原、比對、執行、驗收、合併後快照做不完 |
| **平日 17:00 之後** | `chip_daily_sync` 22:00 → T_iso 21:55、T_open 21:45 | 從 `sr_analysis` 結束到 21:45，大約 4 小時 | ✅ **採用** |
| 週六 | 週日 06:30 的 `stock_symbol_sync` → T_iso 06:25 | 週六 08:45 之後到週日 06:15，大約 21 小時 | 備選：時間最寬裕，但使用者選了平日 |

* 解除封鎖之後，22:00 的 `chip_daily_sync` 和 23:00 的 `sr_analysis_chip` 照常執行，用的就是合併後的資料。17:00 那一輪 `sr_analysis` 是在合併之前跑的，依 D4 補跑。
* 封鎖期間大約 4 小時，盤後的人工操作（交易、持倉）也會被擋。執行日要先告知使用的人。

### 8.3 錯過排程時的補跑

回滾、檢查點 ③ 的超時路徑，或任何讓 backend 在 T_iso 停掉之後才重新啟動的情況，都會錯過晚間的工作；7.4 的重啟關卡為了避開觸發時刻而延後重啟時，跳過的工作也算在內。解除封鎖之後依下面的順序補，**每一個都確認 `job_runs` 是 success，才進下一個**：

1. `chip_daily_sync`（22:00）
2. `sr_analysis_chip`（23:00）：它要用當天的籌碼，所以排在 `chip_daily_sync` 後面

如果連隔天早上都受到影響（例如回滾拖過午夜），隔天的早班工作照下面的順序補：

1. `stock_symbol_sync`（06:30）
2. `delisting_reconcile`（07:00）：它要求主檔的最新一輪同步已經成功，所以排在 `stock_symbol_sync` 後面
3. `corporate_action_sync`（06:30）
4. `pre_market`（08:50）：盡量⛔ 不要讓它也錯過

⚠️ 以下兩點還要讀程式碼確認（E8）：
* 停機期間錯過的 cron 觸發，排程器重新啟動後會不會自動補跑。目前**推定不會**。
* 重跑機制的截止時刻（`JOB_RETRY_CUTOFF`，預設 08:45）對手動補跑有沒有限制。

補跑的入口（手動端點或暫時調整 cron），v8 依確認結果寫成逐步指令。

### 8.4 回滾

* **回滾來源**：7.1 的停寫快照，用 SHA-256 核對過才能使用。B 的話還有切換前保留下來的原 DB。
* **觸發條件**：7.3 第 3 點的任何一項不符，或到了 T_rb 驗收還沒通過。回滾要在 backend 停止的狀態下進行。8.2 的檢查點 ② 不成立時，live 的資料還沒動過，直接恢復服務就好，⛔ 不算回滾。
* **只在解除封鎖之前回滾**。解除之後照 7.4 第 3 點處理，⛔ 不直接還原。
* **回滾後**：live 回到停寫快照的狀態，舊機那段資料仍然缺著。I-131 照常追蹤，修正之後重新演練。

### 8.5 其他風險

| 風險 | 對策 |
|---|---|
| 人工資料搬錯或漏搬 | 逐筆列出，使用者確認之後才執行；演練時逐筆核對 |
| 回滾時丟掉恢復服務之後的寫入 | 驗收全部在封鎖期間完成；解除之後⛔ 不直接還原，改用 7.4 第 3 點 |
| 錯過早班批次 | 8.2 的時間表倒推出開始執行的條件和最晚回滾時刻；超過時依 8.3 的順序補跑 |
| dev 的服務在 smoke 時寫進 `trading`，或打外部資料源 | 6.1：三個服務的 DSN 都改掉，python-worker 不啟動，排程用時間窗隔離，smoke 前後照 2.4 核對，用 `pg_stat_activity` 核對 |
| 排程沒辦法靠開關全停 | 7.3 用時間窗隔離，到了 T_iso 一定停掉 backend；快照前後各做一次事後驗證 |
| 正常啟動的寫入被誤判，或被當成「正常」而放過真正的異常 | 2.4 事先列出預期的變化（migration、`AbortRunning`、註冊驗收帳號），逐項精確核對，其他任何變化都判失敗 |
| 解除封鎖之後的修正混進本次合併造成的變更 | 解除封鎖之前做合併後快照，之後的比對都以它為共同祖先（7.4 第 3 點） |
| 檢查點的判斷用了過期的時間 | 8.2 的三個檢查點都用當下的時間重新判斷 |
| 狀態機表混用兩邊的狀態 | 照 5.3 每個身分只取一邊或重建；演練時驗證 |
| 隱含參照漏改 | 先完成 E5 的盤點；漏改的參照列為演練不通過 |
| dev 的 DB 暴露 live 資料 | 2.5 的 (d)：真實資料只進作業 volume、撤除所有 host port、臨時密碼、核對角色與認證規則；作業期間全程帶著隔離 override；原本的 dev volume 從頭到尾⛔ 不掛載 |
| 主機上的其他帳號直接連 container IP（R5） | dev 是 rootless Docker，container 網段不在主機的 namespace 裡（2.5）；匯入前實測連不上才繼續，連得上就依 D7 停下 |
| 作業期間有人不帶 override 跑了 dev compose | postgres 會換回原本的 volume，真實資料⛔ 不會暴露，但作業中斷；重新帶 override 啟動後重做當下的步驟 |
| 撤 port 或換 volume 的 override 沒有生效（`ports`、`volumes` 是合併的） | 用 `!reset`／`!override` 寫法；每次啟動前用 `scripts/check-isolation-config.py` 檢查最終設定（只印通過／失敗），啟動後用 `docker ps` 和 `ss -ltn` 核對 |
| 比對影響同一台主機上的 live | 用 dev postgres 的資源限制；大量比對避開交易時段 |
| 舊資料只剩一份 | 舊機的 `postgres/data` 和兩份 dump 都保留到本案收斂 |

## 九、測試與驗證策略

* 步驟 1：兩份舊 dump 的差異跟逐表筆數交叉核對（新增 − 刪除 = 筆數差）。
* 給 live 的查詢：每一條都先在 `merge_old` 跑過。
* 三方比對：第一層每個 PK 只落在 U／O／L／S／C 其中一類，五類加起來要等於 PK 聯集的筆數。第二層的撞號、自然鍵重複、參照懸空分開列出，⛔ 不算進第一層的筆數。
* 先用合成資料測比對工具本身：每一類各放幾筆、刪除列、兩邊各自新增同一個 PK、跨 PK 的自然鍵重複，結果要跟預期完全一致。
* E4：在 dev 用 `merge_base` 的複本補跑，跟 `merge_old` 比對，誤差要能逐項解釋。
* 演練：第六節全部通過、腳本重跑結果相同，才交給專人執行。
* smoke：6.1 第 5 點的 `pg_stat_activity` 核對通過，而且第 2 點的事後驗證只出現 2.4 列出的預期變化，smoke 才算有效。
* D5：每次啟動前，`scripts/check-isolation-config.py` 對最終設定的檢查全部通過（沒有 port、postgres／redis 只掛作業 volume、DSN 指向預期的 DB）；啟動後 `docker ps`、`ss -ltn` 確認四個 port 都沒有在監聽；從主機的 namespace 連不到作業 postgres 的 container IP（R5）；角色和 `pg_hba_file_rules` 的查詢結果記進演練紀錄。結束時切回原本的 volume 之後跑 `smoke-dev.sh`，確認 dev 恢復正常。
* 驗證工具本身（v8）：`check-isolation-config.py` 先用合成的 JSON 測試，涵蓋有 port、掛了原本的 volume、DSN 指錯 DB、JSON 解析失敗（輸出⛔ 不能含原始內容）；`smoke-dev.sh` 的修改照 6.1 在沒有真實資料的 dev 上跑三種組合。
* 附錄 A-1：`pg_dump` 失敗時⛔ 不產生正式檔、⛔ 不交接；dev 端還原失敗時⛔ 不進比對。用一次故意失敗的 dump（例如錯的 DB 名稱）在 dev 驗證這條路徑。
* 執行：8.2 的檢查點 ①、② 都成立，而且 7.1 的關卡通過，才執行 7.2。要解除封鎖，必須 7.3 的驗收全部通過、合併後快照已完成、兩次事後驗證都通過；然後依 7.4 第 1 點走正常路徑（T_open 之前）或超時路徑（backend 已在 T_iso 停掉，重新啟動後依 8.3 補跑）。
* 時間窗隔離：執行前，用 E7 的設定逐一算出每個工作的下一次觸發時刻，算出 T_iso，並確認 T_open 早於 T_iso。超時路徑要重啟 backend 之前，照 7.4 的重啟關卡用當下的時間重算一次；演練時用幾個邊界時刻（例如觸發前 3 分鐘、觸發後 1 分鐘）驗證關卡的判斷。
* 作業 Redis 的密碼：`check-isolation-config.py` 的「backend 和 redis 密碼相同、不是 dev 預設值」要有通過和失敗的合成案例；smoke 時 backend log 必須有 `redis connected`。

## 十、執行順序與確認點

1. 使用者確認 v8 → 先完成驗證工具的修改（`smoke-dev.sh`、`check-isolation-config.py`）並在沒有真實資料的 dev 上驗證 → 步驟 1：先照 2.5 完成作業隔離（含 R5 的實測），才匯入任何真實資料。附錄 A 的 A-1（分析快照）和 A-2（設定值）已經在 2.5 之前先執行了：dump 已經收到，放在受限目錄裡，2.5 的隔離完成之前⛔ 不還原；A-2 的結果記在附錄 B。
2. 步驟 1 的報告 → 交出 4.1 的查詢給專人執行 → 4.3 的分析快照 → 三方比對。
3. 同時在 dev 取得 E3～E6、E8；E7 用使用者照附錄 A-2 取回的設定值。
4. 依 E1～E8 寫 v9：裁決 D1～D4，附上逐表的仲裁規則、5.3 選定的做法、7.3 的封鎖指令和 8.2 的時間表 → **使用者確認 v9**。
5. 演練（含 6.1 的 smoke 和第六節第 4 點的計時）→ 把實測時間填進 8.2 → 使用者確認 → 7.1 的關卡 → 7.2 執行 → 7.3 封鎖期間的驗收 → 7.4 解除封鎖。
   * 跟 I-132 的順序（v8）：I-132 的 C（Redis 啟動探測）如果在 I-131 正式合併之前部署，要記下 commit、image id、live `compose.yml`／`deploy.sh` 的 sha；I-131 的最終演練和封鎖期驗收都用同一個版本。版本有變就重做受影響的演練，⛔ 不沿用過期的環境結論。I-132 修好 live 的 `.env` 和 Redis，⛔ 不代表 I-131 的 R1～R5 跟著解決。

## 十一、完成後要歸檔的位置

* `docs/development-workflow.md`：補一節「live 主機搬遷與 DB 合併」，內容包括：
  * 搬遷時先停寫、再做最終 dump 的順序（這次的教訓：用的是停機前一天的 dump）；
  * live 的 log 封存要放在 14 天輪替的範圍之外；
  * live 由專人操作、Claude 只出指令的分工；
  * 用 dev compose 的 postgres 另建 DB 做比對的做法，以及真實資料的保護（2.5：作業專用 volume、撤 port 和換 volume 要用 `!reset`／`!override` 並用腳本檢查最終設定、臨時密碼、全程保護、結束的順序；dev 是 rootless Docker 這個事實和它的保護範圍）；
  * `smoke-dev.sh` 的 `HEALTH_MODE=internal` 和不啟動 python-worker 的服務組合；`check-isolation-config.py` 的用途；
  * 新機的 Docker 版本（29.9.3）：更正 `development-workflow.md` 裡記的舊機版本與 L2 限制的適用範圍；
  * 8.1 的交易語意表；
  * 封鎖期間驗收、解除之後⛔ 不直接還原的原則，以及依早班排程倒推時間表的做法；
  * 排程器沒有整體開關，封鎖要靠時間窗隔離（到了 T_iso 就停掉 backend）加上事後驗證（2.3）；
  * backend 啟動時預期會寫入的變化，以及怎麼精確核對（2.4）；
  * 合併後快照是解除封鎖之後比對的共同祖先；時間表的檢查點一律用當下的時間判斷。
* `docs/issue.md` I-131：改成「已實作／待 review」，記下採用的策略、逐表實際筆數，以及對照表的位置；review 之後收斂。
* 收斂時刪除本計畫書（要先 commit 過），並刪掉所有 `merge_*` DB。

## 附錄 A：交給使用者在 live 上執行的指令

⚠️ 在 live 的帳號上執行。要替換的值一律寫在開頭的變數裡（2026-11-13 已填入實際值）。⛔ 不在指令中間放 `<…>`：zsh 會把 `<` 當成輸入重導向，那一行指令就不會執行（2026-11-13 修 Redis 時實際踩過）。指令執行完，把輸出貼回來；dump 檔照最後一行交給 dev。

### A-1 新 live 的 dump（分析快照；停寫快照用同一套，`PREFIX` 改成 `live_frozen`）

v8 依 v7 review R4 改成：dump 先寫成 `.partial`，**結束碼是 0、檔案不是空的、`pg_restore -l` 讀得了目錄**三個條件都成立，才改成正式檔名、算 SHA-256、交接；任何一個不成立就刪掉 `.partial`，⛔ 不交接。
⚠️ `pg_restore -l` 只能證明檔頭和目錄完整，⛔ 不能代替實際還原。完整性要靠 dev 端的還原（下面的「dev 端」）。
實際交付時，照「指令不用 heredoc」的做法，把下面這段包成 base64 加 sha 核對的形式。

```bash
PG=postgres-postgres-1                  # live 的 postgres container（2026-11-13 附錄 B）
DBUSER=trading_user                     # live 的 DB 使用者（使用者 2026-11-13 確認；舊 dump 的擁有者 trading_username 已改名）
PREFIX=live_analysis                    # 停寫快照改成 live_frozen
OUT=~/i131_live_snap; mkdir -p "$OUT" && chmod 700 "$OUT"
F="$OUT/${PREFIX}_$(date +%Y%m%dT%H%M%S).dump"
date '+start %F %T %z'
if docker exec "$PG" pg_dump -U "$DBUSER" -d trading -Fc > "$F.partial" \
   && [ -s "$F.partial" ] \
   && docker exec -i "$PG" pg_restore -l < "$F.partial" > /dev/null; then
  mv "$F.partial" "$F"
  date '+end %F %T %z'
  sha256sum "$F"; ls -la --time-style=full-iso "$F"
  if sudo install -o dev -g dev -m 600 "$F" /home/dev/i131_merge/live/; then echo "交接完成"; else echo "交接失敗"; fi
else
  echo "dump 失敗，未交接"; rm -f "$F.partial"
fi
```

**預期**：出現「交接完成」，檔案大小跟舊 dump 同一個量級（約 85～90 MB）。`sha256sum` 那一行和開始／結束時間都要貼回來。出現「dump 失敗，未交接」或「交接失敗」時，把那幾行貼回來，⛔ 不重試到成功為止才回報。

**dev 端**：收到後重算 SHA-256 比對；用 `pg_restore -l` 確認檔頭的建立時間和資料表數；接著還原進對應的 `merge_*`，**`pg_restore` 的結束碼不是 0 就停下**，⛔ 不進比對，也⛔ 不把這份當成快照或回滾備份。分析快照和停寫快照（回滾用）都照同樣的規則。

### A-2 設定值（全部唯讀）

```bash
docker version --format 'engine={{.Server.Version}}'; docker compose version
docker ps --format '{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
BE=stock_trading-backend-1; PS=stock_trading-python-server-1; PW=stock_trading-python-worker-1   # 2026-11-13 附錄 B
# 只篩出排程、開關和重跑相關的鍵；⛔ 不印其他環境變數（可能含密碼）
docker inspect "$BE" --format '{{range .Config.Env}}{{println .}}{{end}}' \
  | grep -E '^[A-Z0-9_]*(_CRON|_ENABLED)=|^JOB_RETRY_[A-Z0-9_]*=|^TZ='
docker exec "$BE" sha256sum /app/config.yaml           # 跟 repo 的 backend/config.yaml 比對
docker exec "$BE" sh -c 'grep -hE "scheduler started|register failed|job retry config|redis connected" /app/logs/backend/*.log' | tail -20
docker inspect "$PW" --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -E '^WORKER_POLL_INTERVAL='
docker inspect "$BE" "$PS" "$PW" --format '{{.Name}} ports={{json .HostConfig.PortBindings}} nets={{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}'
```

另外請回報反向代理（caddy）裡指到 stock_trading 的那幾行設定（只要 site 和 `reverse_proxy` 那幾行），以及從外部能不能打到手動觸發的端點。

**預期**：`config.yaml` 的 SHA-256，跟 repo 2026-11-13 的 `backend/config.yaml`（`4bc1c94d…`）一樣，或能說明差在哪裡；
每個 `_CRON`／`_ENABLED` 的值，加上 2.3 列的固定註冊工作，要能算出 7.3 的 T_iso。

## 附錄 B：E7 的回報（2026-11-13，使用者依附錄 A-2 從 live 取回）

**排程**：live 的 `config.yaml` SHA-256 是 `4bc1c94d…`，跟 repo 2026-11-13 的 `backend/config.yaml` 相同。所以環境變數沒有設定的鍵，用的都是 repo 的預設值。

| 工作 | cron（台北時間） | 來源 |
|---|---|---|
| `stock_symbol_sync` | `30 6 * * *`（**每天，含週末**） | 預設（enabled＝true） |
| `delisting_reconcile` | `0 7 * * *`（每天） | env（`DELISTING_ENABLED=true`） |
| `corporate_action_sync` | `30 6 * * 1-5` | env（沒有開關） |
| `pre_market`／`intraday`／`daily_close`（含 `sr_zone_verify`） | `50 8`／`*/5 9-13`／`0 15`，平日 | 固定註冊 |
| `evaluation_universe_sync`（日 K 缺漏偵測跟著它跑） | `0 16 * * 1-5` | env（兩者都 enabled） |
| `sr_analysis`／`sr_analysis_chip` | `0 17`／`0 23`，平日 | env |
| `chip_daily_sync` | `0 22 * * 1-5` | env（沒有開關） |
| `sr_evaluation` | 不註冊 | 預設 disabled |
| 層一的重跑 | 開啟；間隔 10 分鐘、最多 3 次、截止 08:45 | env |

* `TZ=Asia/Taipei`。`WORKER_POLL_INTERVAL` 沒有用 env 設定，用的是 python 端的預設值。
* 由上表算出：D6 選定的平日傍晚窗口是 T0＝`sr_analysis`（17:00）結束之後、T_iso＝21:55、T_open＝21:45；其他時段的比較見 8.2 的 D6。

**對外入口**：

* caddy 只把 `stock.develop-note.com` 反向代理到 `stock_trading-backend-1:8080`。backend 接在 `proxy_net` 和 `trading-net` 兩個網路上，python-server 和 python-worker 只接 `trading-net`。
* 這三個服務都⛔ 沒有發布 host port。live 的 postgres（`postgres-postgres-1`）和 redis（`redis-redis-1`）也只開放容器內部的 port。
* 所以 7.3 封鎖外部寫入的方式，就是改 caddy 這一個 site：例如回維護頁並 reload。手動觸發的端點也走同一個入口，會一起被擋掉。具體指令寫進 v8。

**版本**：live 的 Docker Engine 29.9.3、Compose v5.7.0，跟 dev 查到的相同。

**啟動紀錄**：

* 新 live 的 backend 在 11-12 11:57:46、14:23:34、14:29:51、16:28:37 共啟動四次。14:29:51 那次重跑機制是 `enabled:false`，16:28:37 改回 `true`。
* 11:57:46 那次連的是試跑用的 `trading-test` DB（使用者說明）。改連 `trading` 是從哪一次開始，要在 `merge_live` 的 `job_runs` 裡核對（4.2）。
* ⚠️ **16:28:37 那次啟動 Redis 連線失敗**：`redis connect failed, cache disabled`，錯誤類別是 `NOAUTH`。前三次都是 `redis connected`（使用者用只印訊息名稱和錯誤類別的指令確認）。
  live 從那次之後就在 Redis 停用的狀態下運作；照 i123 計畫書「3.7」的 R5，這屬於部署確認沒有通過。
  這不是本案的範圍，修正由專人處理（使用者 2026-11-13 決定先不另外記錄，修好之後在 I-131 註明）。
  ✅ **2026-11-13 11:21:32 已修正**：原因是 16:28 那次重啟沒有經過 `deploy.sh`，專案目錄的 `.env` 不含 `REDIS_PASSWORD`，所以帶進去的是空字串。
  修法是照 `deploy.sh` 同一套白名單從 Redis 的 `.env` 讀出密碼，只重建 backend。重建後核對結果：image 不變、其餘環境變數的雜湊不變、密碼雜湊跟 Redis 一致、`redis connected`；
  Redis 上看得到 backend 的連線，`indicator:*:latest` 每一輪都有更新；重建沒有中斷任何工作（11:20 那一輪在重建前就結束了）。
  封鎖期驗收的「部署後確認（含 Redis）」照常要做；之後任何重啟都要用同一套方式帶入密碼。

## Review feedback（2026-11-13，v5）

> 狀態：✅ 已在 v6 處理（R1 → 2.5 第 5 點、6.1 第 6 點；D5 → 2.5，建議採 (d)，待使用者確認）。這一節保留原文供追溯，⛔ 不代表已授權建立 DB、修改腳本或操作 live。

前輪的隔離截止、啟動寫入核對與 python-worker 描述已補上。本輪新增一項 D5 相關缺口。

### R1：恢復預設密碼早於真實資料清除（P1）

6.1 的恢復步驟在 smoke 結束後就把密碼改回 `trading/trading`，但 `merge_base`、`merge_old`、`merge_live` 等真實資料庫仍保留。若採 D5(a)，保護會在資料仍存在時被解除。

建議改成：**全部含真實資料的工作 DB 清除並驗證後，才能恢復預設密碼與一般 dev 連線。** 作業失敗或暫停時也要維持保護，⛔ 不能只因 smoke 結束就解除。

### D5 的方案評估與建議

建議新增 **(d)：撤除對外發布＋臨時憑證＋全程保護**，保留原始資料供精確比對。

| 方案 | 評估 |
|---|---|
| (a) 臨時強密碼 | 可作補強，但需涵蓋整個資料留存期間，並清除既有連線 |
| (b) 遮罩 email／密碼雜湊 | 不適合作為合併資料來源；會改變比對內容，也未涵蓋持倉、交易及其他敏感欄位 |
| (c) 限時接受風險 | 只縮短暴露時間，沒有阻止存取 |
| (d) 隔離入口並使用臨時憑證 | 建議採用，避免依賴 localhost 綁定或單一密碼措施 |

建議的處理順序（尚未裁決）：

1. **匯入前先隔離入口。** 沿用 dev compose，透過作業專用設定撤除 PostgreSQL、Redis、backend、Python HTTP 的 host port 發布；停止一般 dev 應用，僅啟動本次必要服務。只撤 DB port 仍可能透過應用 API 讀到資料。
2. **更換 dev DB 臨時隨機密碼並終止既有應用連線。** 要實際修改 PostgreSQL 角色；既有 volume 不會因修改 `POSTGRES_PASSWORD` 就換密碼。憑證放在 repo 外的受限檔案中，⛔ 不印到輸出或提交版控。依據：[官方 image 的環境變數說明](https://github.com/docker-library/docs/blob/master/postgres/README.md#environment-variables)。
3. **核對角色與認證規則。** 此 compose 的 `POSTGRES_USER=trading` 依官方 image 初始化方式會建立 superuser，實際現況仍須查證。若它仍是 superuser，僅對 `merge_*` 撤銷一般權限不足以隔離它。需核對實際角色與 `pg_hba.conf`，再定作業期間允許的連線。依據：[PostgreSQL 角色權限](https://www.postgresql.org/docs/16/role-attributes.html)、[認證規則](https://www.postgresql.org/docs/16/auth-pg-hba-conf.html)。
4. **讓 smoke 配合作業隔離。** 現有腳本預設從 host port 做 health check，需先把內部檢查方式納入確認後的腳本方案，再依 repo 規範實作與驗證。Compose 的 port 清除也要核對最終設定，⛔ 不能假設 override 寫 `ports: []` 就會移除原設定；語法須符合實際 Compose 版本。依據：[Compose 合併規則](https://docs.docker.com/reference/compose-file/merge/)。
5. **保護並清理所有副本。** dump、差異明細、ID 對照及憑證存 repo 外受限目錄；作業失敗或暫停時繼續維持隔離。清除所有工作 DB、相關快取及非必要輸出後，才恢復一般 dev 設定；需留存的證據仍依原保存規則保護，⛔ 不因本項建議提前刪除。

**保護邊界與驗證限制**：這個方案保護的是網路與一般 dev 存取；擁有主機 root／Docker 管理權的人仍屬可信任管理者範圍。新機的 Docker／Compose 版本、實際角色及 `pg_hba.conf` 本輪尚未核驗，⛔ 不把舊機的版本紀錄當成新機實測。本輪只做靜態 review，未修改執行設定或操作 DB／live；`git diff --check` 通過。

## Review feedback（2026-11-13，v6）

> 狀態：✅ 已在 v8 處理（R1 → 2.5 改成作業專用 volume，原本的 cluster 不再恢復公開帳密；R2 → 6.1 改用 `smoke-dev.sh` 並補上內部 health 模式；R3 → 2.5 第 2 點的 `check-isolation-config.py`；P3 → `issue.md` I-131 的歷史段落加註）。以下保留原文。原始狀態：待處理。v5 R1 的「資料仍保留就恢復預設密碼」已由 2.5、6.1 修正；本輪另有下列三項。僅記錄 review，⛔ 不代表修改方案已確認或已授權執行。

### R1：恢復公開密碼的 superuser 仍可讀取殘留 WAL（P1）

2.5 第 5 點在 `DROP DATABASE`、`CHECKPOINT` 後恢復 `trading/trading` 和 host port；同節又明確保留 WAL 殘留的限制。如果實際 `trading` 仍是 superuser，任何取得這組公開帳密的一般 dev 使用者，都能透過 SQL 的 `pg_read_binary_file` 讀 PostgreSQL 可存取的檔案，包括尚留在資料目錄裡的 WAL。這條路徑不需要主機 root 或 Docker 管理權，所以不能只用「可信任主機管理者」的保護邊界排除它。`CHECKPOINT` 也不能作為本計畫所列的殘留資料已清除證明。

建議：把恢復一般 dev 存取的權限條件明確列成關卡。若保留此 cluster，特權管理角色繼續用受限憑證；一般 dev 連線改用沒有 superuser、讀取伺服器檔案或繼承相關特權的角色，並驗證無法讀取殘留檔案。另一候選是沿用 dev compose 的拋棄式作業資料卷，作業後切回未匯入真實資料的原 dev 資料卷；這會改變現行方案，需先確認、保全原 dev 資料。⛔ 不直接清空原 dev 或 live volume。

實際角色尚待查證；本項要求的是計畫不能在未驗證權限邊界前，恢復公開的特權帳密。依據：[PostgreSQL 16 的 Generic File Access Functions](https://www.postgresql.org/docs/16/functions-admin.html#FUNCTIONS-ADMIN-GENFILE)。

### R2：以一次性 Docker 指令取代 smoke 腳本，違反驗證規範（P2）

6.1 明定不用也不修改 `smoke-dev.sh`，改直接下 compose 指令做正式演練驗收；但 `development-workflow.md`「測試腳本優先」要求：現有腳本涵蓋不到需求時，先確認需求、補進腳本，再用腳本驗證；一次性指令僅作診斷。撤除 host port 是合理的隔離需求，不代表可以解除腳本規範。

建議：把不發布 port 的內部 health／API 檢查能力納入本案待確認的腳本變更，保留既有 dev project 保護、相依服務 readiness、逾時及失敗診斷；驗收仍由 repo 腳本執行。若保持不改腳本，就必須先取得符合既有腳本能力的替代驗證方案，⛔ 直接把診斷指令當正式驗收。

### R3：直接印出 compose config 會洩漏臨時憑證（P2）

2.5 第 1 點、6.1 第 3 點用不帶輸出限制的 `docker compose … config` 核對 port，而同一個 override 已包含臨時密碼的 `DATABASE_DSN`。`config` 預設會將解析後的完整設定印到 stdout，會把 DSN 以及其他環境變數帶進終端、執行紀錄或回貼輸出，違反 2.5「憑證不印到輸出」的要求。

建議：由檢查工具在記憶體解析 `config --format json`，只輸出服務名稱、是否發布 port 與通過／失敗；錯誤路徑也⛔ 回顯原始設定或憑證。`config -q` 可做語法驗證，但⛔ 單獨證明沒有 port。依據：[Docker compose config](https://docs.docker.com/reference/cli/docker/compose/config/)。

### 文件一致性（P3）

`issue.md` I-131 同時保留「v5 已處理」和另一段「v5 一項 P1 待處理」的現行描述。應保留歷史原文並清楚加註已由 v6 處理，把本輪尚未處理的項目另列，避免把新舊缺口混為一談。

本輪為靜態 review，未建立 DB、未操作 Docker／live；`git diff --check` 通過。

## Review feedback（2026-11-13，v7）

> 狀態：✅ 已在 v8 處理（R1～R3 同上一節；R4 → 附錄 A-1 改成 `.partial`、三個條件都成立才交接，dev 端還原失敗就停下；P3 同上一節）。以下保留原文。原始狀態：待處理。v7 補上 dump 交接、schema 擁有者／權限差異、E7 回報與平日傍晚窗口，但未處理 v6 feedback 的三個阻擋項。這一節只記錄 review，⛔ 不代表授權建立 DB、操作 live 或執行附錄指令。

### 前輪 R1～R3：仍未修正（沿用一項 P1、兩項 P2）

* **R1（P1）**：2.5 仍在 `DROP DATABASE`、`CHECKPOINT` 後恢復公開的 `trading/trading`，並接受 WAL 殘留；若該帳號仍是 superuser，一般 dev 連線就能透過 SQL 讀取殘留檔案。尚未加入恢復連線前的權限關卡或改採作業專用資料卷。
* **R2（P2）**：6.1 仍明定不用也不修改 `smoke-dev.sh`，以一次性 compose 指令做正式驗收，與 repo 的腳本優先規範衝突。尚未提出符合規範的內部 health／API 驗證腳本方案。
* **R3（P2）**：2.5、6.1 仍直接執行完整的 `compose config`，但同一個 override 裡已有含臨時密碼的 DSN；尚未改成只輸出白名單欄位的檢查方式。

完整成因、建議與官方依據沿用前一節「Review feedback」v6。以上是既有未解項目，⛔ 不重算成三個新發現。

### R4：dump 失敗仍會繼續交接檔案（P2，新增）

附錄 A-1 的 `pg_dump` 之後只有 `echo "rc=$?"`，沒有失敗即中止的控制。即使 `pg_dump` 失敗，後面的 `date`、`sha256sum`、`ls` 和 `sudo install` 仍會繼續，將空檔或部分 dump 交給 dev。檔案能算 SHA-256 只能證明交接內容相同，⛔ 不代表備份完整；`pg_restore -l` 也只檢查目錄，不能取代實際還原驗證。

~~而且 A-1 的 `DBUSER` 預設仍是舊機的 `trading_username`，與 v7 已記錄的新 live 擁有者 `trading_user` 不一致，增加指令直接失敗的機會。~~ **同版複查：帳號部分已修正**，A-1 現在明列 `PG=postgres-postgres-1`、`DBUSER=trading_user`，並記載使用者已確認。物件擁有者不一定等於可登入的備份角色，應以確認結果為準。R4 的失敗不中止問題仍未修正。

建議：dump 先寫暫存檔，明確檢查結束碼，失敗就中止且不發布、不交接成正式快照；成功後才計算雜湊、改為正式檔名並交接。dev 端還原也必須檢查結束碼並在失敗時停止後續比對。附錄既然同時用於分析快照與回滾備份，兩種用途都要採同樣的失敗處理。

### 文件狀態（沿用 P3）

I-131 仍同時出現 v5「已處理」和「待處理」的現行描述，需按 v6 feedback 的要求同步。~~另附錄 B 已記錄 Redis `NOAUTH`，目前仍是 live 執行前的明確阻擋，不能把已收集 E7 等同部署確認通過。~~ **同版複查：附錄 B 已補記 2026-11-13 11:21:32 修復**，含 `redis connected`、backend 連線與 cache 更新的確認紀錄。這項歷史故障不再列為尚未修復的阻擋；I-131 執行當次仍須重做部署確認。這次 review 沿用附錄紀錄，未自行連 live 核驗。

**v7 同版複查結論**：本次未新增發現。R1（P1）、R2／R3（P2）仍未修正；R4（P2）已修正帳號，但 dump 失敗後仍繼續交接，故保持待處理。P3 的新舊狀態矛盾仍在。以上更新既有 feedback，不另發重複項目。

本輪為靜態 review；未重新驗證 dump、未執行附錄指令或操作 DB／live。`git diff --check` 通過。

## 整合評估：D5(d) 與 I-132（2026-11-13，v7）

> 狀態：✅ 已在 v8 處理（第 1 點 → 2.5 的作業專用 volume；第 2 點 R5 → 2.5 查到 dev 是 rootless Docker、container 網段不在主機的 namespace 裡，改成匯入前實測，連得上就依 D7 停下；第 3 點 → 6.1；第 4 點 → 2.5 第 2 點；第 5 點 → redis 也換作業 volume；與 I-132 的順序 → 第十節第 5 點）。以下保留原文。原始狀態：建議／待納入下一版。本輪評估 I-132 v2 和 I-131 v7 的銜接，未實作或操作 live；不將其他代理的文字或本 review 當成使用者已確認的裁決。

**v7 review 狀態**：R1～R4 仍待處理（一項 P1、三項 P2）；A-1 的帳號已修正，dump 失敗不中止尚未修正。Redis 的歷史 NOAUTH 沿用附錄 B 的修復紀錄，不重列為未修復問題。I-132 的 repo 範本、`.env.example` 與 C 修正，也不會自動解除這四項。

**D5(d) 建議採用，但必須補齊以下具體設計**：

1. **用作業專用 PostgreSQL volume，沿用 dev compose／project**。原 dev volume 保留且不掛進本次作業 PostgreSQL；作業用的 PostgreSQL 資料目錄必須只對應新的具名 volume，例如 `stock_trading_dev_i131_pgdata`，不能同時掛載原資料卷。所有 `merge_*` 都在作業卷裡。用臨時隨機憑證初始化／保護，結束後先停作業服務，切回未匯入 live 資料的原 dev volume，才恢復原 dev 的一般存取。作業卷的刪除與證據保留時點待 review 確認，⛔ 用 `down -v` 一併刪掉原 dev volume。這能把殘留 WAL 與一般 dev 的公開 superuser 帳密分開，無須假設 `CHECKPOINT` 已清除機密。這是對 2.2 現行做法的變更，要寫入下一版再確認。
2. **撤掉發布 port 仍是必要措施，但不足以隔離同機帳號（R5，P2，新增）**。Docker bridge 預設允許主機直接連 container IP，不發布 host port 不會移除這條路徑。2.5 已把同機其他帳號列入威脅範圍，因此只驗 `docker ps`／`ss` 沒有四個 port 不足以通過。先前將撤 port 視為能涵蓋一般同機存取的建議需補正。DB 必須驗證認證規則和臨時密碼；backend／Python HTTP／Redis 也要處理直接連 IP 的路徑。特別是目前 Python HTTP 服務在 container 內綁 `0.0.0.0`，不能靠 backend 的 JWT 保護它。由專人規劃局限於作業 dev 網段的 host 存取限制並驗證，或提出等效的服務認證／隔離；沒有這個條件就不能宣稱防住同機不可信帳號。不得順手改動 live 網路或全域防火牆。依據：[Docker bridge 的主機存取預設行為](https://docs.docker.com/engine/network/drivers/bridge/)。本輪未實測主機現行網路規則，故列為待驗證的必要關卡。
3. **內部 smoke 能力納入 repo 腳本的待確認變更**，保留 project、相依服務、逾時與失敗診斷的保護；不能以「這次不改腳本」來繞過既有驗證規範。沿用 dev compose，驗證方式與工作資料卷的參數也須一併可審查。
4. **配置檢查只輸出必要的白名單結果**。記憶體解析最終 compose model，檢查 port、實際掛載卷及目標 DB；不印完整 config、DSN 或憑證。錯誤路徑也不能回顯原始設定。
5. **資料清理涵蓋副本與快取**。dump、差異報告、ID 對照、設定與憑證留在受限目錄；作業專用 cache 也不能把 live 內容帶回一般 dev Redis。保留的作業卷／證據持續受限，失敗或暫停不解除保護。原 dev 的持久資料不作破壞性重置。

**與 I-132 的順序**：先把 I-132 的 v3 範圍、D1～D6 和 Redis 啟動時間上限整理成可實作的計畫；I-131 另把 D5(d)、R1～R5 及腳本改動收斂到下一版。I-132 的程式部署如安排在 I-131 正式合併之前，要記錄 image／設定版本，I-131 的最終演練與驗收使用相同版本，不能沿用已過期的環境結論。

## 十二、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-13 | 初版 |
| v2 | 2026-11-13 | 依 v1 的 review 修正：①兩份舊 dump 的差異只代表「舊機這段期間的變更」，改成以 `merge_base` 為共同祖先的三方比對（4.4），加上 id 撞號與自然鍵重複兩類；②撤回「初步建議 B」——B 的前提（歷史 SR 和狀態機可以補跑）尚未驗證，改列為證據 E4，A／B 都保留到 v3 依證據裁決；補上 E1～E6、狀態機的處理（5.3）、id 與參照的對照（4.5，含 FK 清單和尚未盤點的隱含參照）；③另起 container 跟「開發驗收只用 dev compose」衝突 → 改成在 dev compose 的 postgres 另建 `merge_*` DB，並加上真實資料的暴露風險（D5）；④v1 宣稱「單一 transaction 保證回滾」不正確——`nextval`／`setval` 是非交易性的，整庫重建的 `DROP／CREATE DATABASE` 不能放進 transaction → 補上 8.1 的交易語意表、停寫快照與執行前的關卡（7.1）、回滾的觸發條件與期限（8.2）；⑤加上執行順序與確認點（第十節），v3 裁決前⛔ 不進演練 |
| v3 | 2026-11-13 | 依 v2 的 review 修正：①[P1] 先啟動服務、再確認和補跑，回滾時會丟掉恢復服務之後的寫入 → 改成驗收全部在封鎖期間完成（7.3：封鎖外部寫入、排程和佇列，怎麼封鎖要等 E7），解除之後⛔ 不再用停寫快照回滾，改成先保全、再用三方比對修正（7.4）；②[P2] 4.4 的分類漏了未變更的列，撞號同時屬於兩類，自然鍵重複可以跨 PK，第九節的「每列只屬一類」不成立 → 改成兩層：第一層以 PK 做互斥分類 U／O／L／S／C，並訂出刪除與新增的表示方式；第二層的撞號、自然鍵重複、參照懸空另外列，並補上用合成資料測比對工具；③[P2] dev compose 的三個服務都寫死連到 `trading`，直接跑 smoke 驗到的是原本的 DB → 補上 6.1：用既有的 `OVERRIDE` 機制讓三個服務一起指向演練 DB，關掉排程，用 `pg_stat_activity` 核對實際連線，並寫出恢復步驟；④[P2] 08:00 的回滾期限晚於 06:30／07:00 的早班批次，也沒有預留還原時間 → 改成依實測時間倒推（8.2：T0、T_open 06:15、T_rb、開始條件），補上錯過時的補跑順序（8.3），並新增證據 E7、E8；章節重編為 8.1～8.5 |
| v4 | 2026-11-13 | 依 v3 的 review 修正：①[P2] 「關掉所有排程開關」做不到排程全停——backend 無條件啟動排程器，`pre_market`、`intraday`、`daily_close`、`chip_daily_sync` 無條件註冊，`corporate_action_sync` 也沒有開關 → 新增 2.3（讀程式碼確認的排程事實），dev 的 smoke 和 live 的封鎖都改成時間窗隔離加上事後驗證（`job_runs` 不能有新的列），E7 擴大為每個已註冊工作的 cron 和開關的實際值；②[P2] 解除封鎖之後用停寫快照當比對起點，會混進本次合併造成的變更 → 驗收通過之後、解除封鎖之前做合併後快照，並記下 SHA-256（7.3 第 3 點第 4 步），之後的比對都以它為共同祖先；③[P2] 開始條件漏算停寫快照、交接、還原與比對的時間，T0 也⛔ 不代表實際進度 → 8.2 改成三個檢查點，每一個都用當下的時間重新判斷，並列出各段時間的代號和量測方式；④第十節的版次改成 v4。依證據裁決策略的版本順延為 v5 |
| v5 | 2026-11-13 | 依 v4 的 review 修正：①[P2] 檢查點 ③ 的超時分支讓 backend 繼續跑，06:30 的排程會照常觸發，⛔ 不是「錯過」；而且事後驗證只在快照之前做，抓不到快照期間觸發的工作 → 新增隔離截止時刻 T_iso（最早的觸發時刻 − 5 分鐘），到了就一定停掉 backend；事後驗證改成快照前後各做一次；7.4 第 1 點統一成正常路徑和超時路徑兩條，第九節同步；②[P2] backend 啟動時本來就會執行 `AbortRunning`、自動套用 migration，「雜湊不能變」會把正常啟動誤判成失敗 → 新增 2.4，事先列出預期的變化（含註冊驗收帳號），逐項精確核對，其他變化仍然判失敗；③更正 2.3：python-worker 是輪詢 `backtest_jobs` 裡 pending 的回測任務，⛔ 不是消化 Redis 佇列（保持 worker 停止的做法不變）。依證據裁決策略的版本順延為 v6 |
| v6 | 2026-11-13 | 依 v5 的 review（「Review feedback（2026-11-13，v5）」）修正：①[P1] R1：6.1 在 smoke 結束後就恢復預設密碼，這時含真實資料的 `merge_*` 還在 → 改成保護要涵蓋整個資料留存期間（含失敗、暫停、跨天），`merge_*` 清除並驗證、`CHECKPOINT`、清掉非必要輸出之後，才恢復密碼和一般 dev 設定（2.5 第 5 點）；6.1 第 6 點改成 smoke 之後⛔ 不恢復；②D5 建議採 (d)：撤除所有 host port（用 reset 寫法並檢查最終設定）、臨時密碼加上切斷既有連線、核對角色與 `pg_hba`、全程保護；(b)、(c) 不採用（新增 2.5）；③撤 port 之後 `smoke-dev.sh` 的 host health check 不能用 → 6.1 改成直接下 compose 指令、從 container 內部檢查，⛔ 不修改 `smoke-dev.sh`；python-worker 全程不啟動；④記下新機實測的 Docker Engine 29.9.3、Compose v5.7.0，`development-workflow.md` 記的是舊機的版本，列入歸檔時更正。依證據裁決策略的版本順延為 v7 |
| v7 | 2026-11-13 | （同日補充二：D6 由使用者定案為平日 17:00 之後 → 7.3、8.2、8.3、附錄 B 改成傍晚窗口：T0＝`sr_analysis` 結束之後、T_iso 21:55、T_open 21:45，補跑順序改成 `chip_daily_sync` → `sr_analysis_chip`；14:00 因為不到 1 小時而不採用，週六留作備選；新增 E9。更正：新 live 11-12 11:57 那次連的是 `trading-test`，⛔ 不是並行寫入正式資料；16:28 那次 Redis 確定是 NOAUTH）（同日補充：附錄 B 記下 E7 的回報，包括排程表、T_iso＝06:25、caddy 是唯一的對外入口、新 live 11-12 11:57 就啟動了、16:28 那次啟動沒有 Redis 狀態行；2.2 和 5.1 補上 schema 擁有者與權限的差異；8.2 新增 D6，建議週六執行）流程調整（使用者 2026-11-13 定案）：取得 live 資料一律由使用者照 Claude 的指令 `pg_dump` 後交給 dev 的受限目錄 `~/i131_merge/`，查詢改在 dev 的 `merge_live` 上跑，⛔ 不在 live 上跑 SQL（2.1、4.1～4.3、7.1）；程式參數由使用者照唯讀的 docker 指令取出後貼回（E7）；新增附錄 A（A-1 dump、A-2 設定值）；「第一次開始寫入的時間」改從 `merge_live` 推算。依證據裁決策略的版本順延為 v8 |
| v8 | 2026-11-13 | 依 v6、v7 的 review 和 D5(d) 的整合評估修正（逐項判斷，全部採納，R5 依查到的事實降級）：①[P1] R1：2.5 改成**作業專用資料卷**（`stock_trading_dev_i131_pgdata`／`_redisdata`，用 `volumes: !override`），原本的 dev volume 從頭到尾⛔ 不掛載；新 volume 初始化時就用臨時密碼，⛔ 不再 `ALTER ROLE`；刪掉「`DROP DATABASE`、`CHECKPOINT` 之後恢復 `trading/trading`」整段，結束時切回原本的 volume，作業 volume 依新的 D8 整個刪除；2026-11-13 在 dev 用 `config` 驗證過 `!reset`／`!override` 的效果；②[P2] R2：6.1 改用 `smoke-dev.sh`，補上 `HEALTH_MODE=internal`，並讓明確指定 `SERVICES=backend,python-server` 時⛔ 不啟動 python-worker（現在兩個都選會進 `all` 模式，把 worker 一起起來）；預設行為不變；③[P2] R3：新增 `scripts/check-isolation-config.py`，在記憶體裡解析最終設定，只印通過／失敗；④[P2] R4：附錄 A-1 改成 `.partial`，結束碼、非空、`pg_restore -l` 都通過才交接，dev 端還原失敗就停下；⑤[P2] R5：2.5 記下 dev 是 rootless Docker、主機的 namespace 看不到 dev 的網段、主機能登入的帳號只有 root／kirai／dev／app，所以同機帳號只能經由發布的 port 進來；改成匯入前實測連不到 container IP 才繼續，連得上就依新的 D7 停下；⑥redis 也換作業 volume，快取⛔ 不會帶回一般 dev；⑦一、不做：產品程式不改，驗證工具的修改列入本計畫；⑧第十節加上跟 I-132 的版本協調；⑨P3：`issue.md` I-131 的 v5 歷史段落加註已由 v6 處理。依證據裁決策略的版本順延為 v9。**同日補充（v8 review，四項 P2 裡屬於本計畫的兩項）**：①6.1 的 override 也要把 backend 的 `REDIS_PASSWORD` 換成作業 redis 的臨時密碼，`check-isolation-config.py` 檢查兩邊相同而且不是 `dev-redis-only`，smoke 要看到 `redis connected`；②7.4 的超時路徑加上重啟關卡：用當下的時間重算下一次觸發時刻，不夠開放就讓 backend 保持停止，等觸發時刻過了再算，跳過的工作進 8.3 |
