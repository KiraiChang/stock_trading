# Redis 與 dev compose 的密碼、port 收斂 計畫 v1.3（2026-11-03，✅ D1～D4 使用者確認；v1.1～v1.3 是 review 第 1～3 輪的修正）

需求（使用者 2026-11-03）：repo 裡的 Redis compose 範本把 Redis 以⛔ 沒有密碼、綁 `0.0.0.0` 的方式發布，live 照這個範本部署，
曾因此對外暴露（2026-11-03 已在 live 手動修正：拿掉對外 port、加上密碼、backend 帶上密碼）。這次把 repo 的範本與 dev compose 收斂成同樣的安全預設。
範圍（使用者確認）：repo 的 `docker-compose.yml`、Redis compose 範本、dev compose 只綁本機、dev 的 Redis 也加密碼。

## 一、目標與不做的範圍

* **G1** repo 的 `docker-compose.redis.yml` 與 live 現況一致：⛔ 沒有對外 port、密碼從 `.env` 帶入、經 image 的 entrypoint 降權成 redis、healthcheck 帶認證、有資源上限。
* **G2** repo 的 `docker-compose.yml` 與 `deploy.sh` 範本讓 backend 能帶上 Redis 密碼（`REDIS_PASSWORD`），沒設時維持現在的行為（空字串）。
* **G3** `docker-compose.dev.yml` 的 port 一律只綁 `127.0.0.1`；dev 的 Redis 也要求密碼，dev backend 帶上。
* **G4** 文件：部署段落與 dev 的說明照實更新。

不做：
* ⛔ 不改 live 的任何檔案或 container（live 已由使用者手動修正；repo 的範本⛔ 不會自動影響 live）。
* ⛔ 不改程式（backend 本來就讀 `REDIS_PASSWORD`）。
* ⛔ 不處理 sshd、其他 stack（例如 live 的 postgres）的暴露——另外決定。
* ⛔ 不在 repo 記錄這次暴露的事件細節（使用者決定）。

## 二、做法

### A. `docker-compose.redis.yml`（live Redis stack 的範本）

* 拿掉 `ports:`：backend 經 `trading-net` 用 `redis:6379` 連，⛔ 不需要 host 上的 port。檔頭註明：真的要從 host 連，改成只綁 `127.0.0.1`。
* `command: ["sh", "-c", "exec docker-entrypoint.sh redis-server --requirepass \"$$REDIS_PASSWORD\""]`：
  * 經 `docker-entrypoint.sh`：image 的 entrypoint 只在第一個參數是 `redis-server`、身分是 root 時才 `chown /data` 並 `setpriv` 成 redis；
    直接 `sh -c "exec redis-server …"` 會繞過它、以 root 執行（2026-11-03 在 live 踩過）。
  * 用 `sh -c` 而⛔ 不是把密碼直接放進 `command`：`docker inspect` 看到的只是 `$REDIS_PASSWORD` 的字面；Redis 會改寫自己的行程名稱，`ps` 也看不到。
* `environment`：`REDIS_PASSWORD: ${REDIS_PASSWORD:?REDIS_PASSWORD is required}`（沒設就讓 compose 報錯，⛔ 不會起一個沒有密碼的 Redis）、
  `REDISCLI_AUTH: ${REDIS_PASSWORD}`（container 裡的 `redis-cli` 自動認證）。
* healthcheck：`["CMD-SHELL", "redis-cli ping | grep -q PONG"]`（`redis-cli` 遇到認證錯誤時結束碼仍可能是 0，所以比對 `PONG`）。
* 補資源上限：`cpus: "0.5"`、`mem_limit: 512m`、`memswap_limit: 768m`、`pids_limit: 200`（與 live 及 `development-workflow.md`「每個 stack 的 compose 都要有」一致）。
* 檔頭註解：上面的設計理由，以及 `.env` 的建法（compose 檔所在目錄、權限 600、用 `python3 -c 'import secrets…'` 直接寫入、⛔ 不 `cat`）。

### B. `docker-compose.yml` 與 `deploy.sh`（live app 的範本）

* backend 的 `REDIS_PASSWORD: ""` 改成 `REDIS_PASSWORD: ${REDIS_PASSWORD:-}`。沒設時仍是空字串，行為與現在相同。
* `deploy.sh` 範本加 `export REDIS_PASSWORD`，註解寫明值要與 Redis stack 的 `.env` 一致，並附上從那份 `.env` 讀值的寫法（⛔ 不把值寫進範本）。
  * （v1.1）讀值要 fail-closed：讀不到檔案、值是空的、鍵重複（sed 會印兩行）都在 `down` 之前 `exit 1`，live 的 container 照常跑。
    ⛔ 不能寫成 `export X="$(sed …)"`：那樣的結束碼是 `export` 的，sed 失敗會被吞掉；檔案在但沒有這個鍵時 sed 本來就回 0，所以還要檢查值。錯誤訊息只印路徑。
  * （v1.2）檢查的是原始檔，⛔ 不能只檢查 `$(…)` 的輸出（它會吃掉結尾的換行）：凡是 compose 可能當成這個鍵的行（行首空白、`export` 開頭、`=` 前後有空白、只寫鍵名）
    都算，要剛好一行；那一行要寫成 `REDIS_PASSWORD=<值>`，值只用英數與 `._+/=-`——其他字元 compose 可能另有解讀，擋下才保證兩邊讀到同一個值。
  * （v1.3，取代 v1.2 的計數法）整份 `.env` 走白名單，⛔ 不模仿 compose 的解析：只允許空行、從第一個字元就是 `#` 的註解、剛好一行 `REDIS_PASSWORD=<值>`
    （值同上），其他任何一行都擋下。v1.2 只計「認得的」寫法、其他行忽略，漏掉了 compose 也接受的 `KEY: value`。

### C. `docker-compose.dev.yml`

* 四個 port 都改成只綁本機：`127.0.0.1:15432:5432`、`127.0.0.1:16379:6379`、`127.0.0.1:18080:8080`、`127.0.0.1:18001:8001`。
  `scripts/smoke-dev.sh` 與文件裡的 `localhost:18080` 等用法都⛔ 不受影響。
* dev 的 Redis 加密碼：固定的 dev 預設值（D2），Redis 與 dev backend 用同一個變數；healthcheck 同 A。
  dev 的 Redis 已經只綁本機，密碼是第二層防護；值和 dev postgres 的 `trading/trading` 一樣寫在 compose 裡（dev 專用，⛔ 不是 secret）。
  ⚠️（v1.1）這個前提變弱了：這台的 Docker（`20.10.5+dfsg1`）早於 28.0.0，只綁本機擋不住同一個 L2 網段的主機（見十「review 第 1 輪」），
  而寫在 repo 裡的預設密碼對這種情況⛔ 不算保護。D2 維持，補強另行裁決。

### D. 文件

* `docs/architecture.md`「生產 / Docker」與 `docs/development-guide.md`「Docker Compose（生產環境）」：
  * Redis 要先建 `.env`，`docker-compose.redis.yml` 才起得來；backend 要帶 `REDIS_PASSWORD`。
  * 更正 postgres 的描述（D4）：現在的 `docker-compose.postgres.yml` 是 migration 驗證用的 pgtest（獨立 project 與 network），⛔ 不是正式的 postgres，也⛔ 不會建立 `trading-net`。
* `docs/development-workflow.md`：
  * 新增「live 的 Redis：獨立 stack、密碼與 port」——Redis 是獨立的 stack（範本是 repo 的 `docker-compose.redis.yml`），⛔ 不在 stock_trading 的 compose 裡；
    密碼在那個 stack 的 `.env`，backend 的 `REDIS_PASSWORD` 要一致；backend 只在啟動時連一次（失敗就整個停用 Redis，⛔ 不標 degraded），改密碼後要重新部署 backend；
    確認方法（`redis connected`、密碼只看長度）。
  * dev 的 port 只綁本機的說明，以及 dev Redis 的密碼。

## 三、決定（✅ 2026-11-03：D1 只綁本機、文件附 SSH tunnel——使用者目前就是 SSH 進 host 操作 dev；D2～D4 依建議）

| # | 問題 | 選項 | 建議 |
|---|---|---|---|
| D1 | dev 的 port 只綁本機後，從別台電腦就連不到 dev 的 UI／API | 只綁本機（要從別台看就用 SSH tunnel）／維持 `0.0.0.0` | 只綁本機（使用者已選）；文件附 SSH tunnel 的寫法 |
| D2 | dev Redis 的密碼 | 寫死在 dev compose 的預設值（可用環境變數覆寫）／要求另建 `.env` | 寫死的預設值：dev 專用、⛔ 不需要多一個檔案 |
| D3 | Redis 範本要不要留 port | ⛔ 不留（與 live 一致）／只綁 `127.0.0.1` | ⛔ 不留；檔頭寫明要從 host 連時改成 `127.0.0.1` |
| D4 | 兩份文件的 postgres 描述與現況不符 | 這次一起更正／只改 Redis 的部分 | 一起更正：同一段要改，留著會讓人照錯的指令起錯的 postgres |

## 四、受影響的檔案與資料流

* `docker-compose.redis.yml`、`docker-compose.yml`、`deploy.sh`、`docker-compose.dev.yml`。
* `docs/architecture.md`、`docs/development-guide.md`、`docs/development-workflow.md`。
* 資料流：backend → Redis 多了認證（`REDIS_PASSWORD`）。dev 的 backend 與 Redis 用同一個 dev 預設密碼。live ⛔ 不受影響（範本⛔ 不會自動套用）。

## 五、風險與回滾

| 風險 | 對策／回滾 |
|---|---|
| 照新範本起 Redis 時忘了建 `.env` | `:?` 讓 compose 直接報錯，⛔ 不會起一個沒有密碼的 Redis；檔頭寫明建法 |
| backend 沒帶到密碼 | 啟動 log 會是 `redis connect failed, cache disabled`；文件寫明確認方法。回滾：`REDIS_PASSWORD` 留空時與現在相同 |
| dev 從別台電腦連不到 | SSH tunnel；回滾：把 port 改回不帶 `127.0.0.1` |
| dev 的既有 Redis 資料 | dev 的 Redis volume 照用；改成要求密碼⛔ 不影響資料 |
| （v1.1）`deploy.sh` 讀不到 Redis 密碼 | 在 `down` 之前 `exit 1`，live ⛔ 不會被換成一個連不上 Redis 的 backend |
| （v1.1）Docker 28.0.0 以前只綁 `127.0.0.1` ⛔ 不是硬界線 | 記成已知限制（`development-workflow.md`「Docker 環境區隔」）；補強另行裁決 |

## 六、測試與驗證

* `docker compose -f <檔> config --no-interpolate`：四份 compose 都解析得過。Redis 範本：⛔ 沒設 `REDIS_PASSWORD` 時 `config` 報錯；有設時通過。
* Redis 範本與 live 現況比對：結構相同（遮掉值之後 diff，預期只差註解）。live 已經以同樣的設定運作（2026-11-03 確認：以 uid 999 執行、帶密碼 `PONG`、不帶被拒、host 上⛔ 沒有 6379）。
  ⛔ 不另外起一個 Redis 來測範本：範本的 network 名稱就是 live 的 `trading-net`。
* dev 實跑（不 build）：`NO_BUILD=1 scripts/smoke-dev.sh`（image 是 11-02 建的，程式沒變）→ health 通過；dev Redis 不帶密碼被拒、帶密碼 `PONG`；
  dev backend 的啟動 log 是 `redis connected`；`ss -ltn` 只看得到 `127.0.0.1` 的 15432／16379／18080／18001。驗完 `down`（⛔ 不帶 `-v`）。
  ⚠️（v1.1）`ss` 只能證明監聽的位址，⛔ 不能證明外部連不到（Docker 28.0.0 以前的限制）。
* （v1.1）`deploy.sh` 讀密碼：在 scratchpad 複製一份，換掉 `.env` 的路徑，用只記錄參數的假 `docker` 跑——檔案不存在、沒有讀取權限、空檔、沒有這個鍵、鍵重複都要 rc 1 且 docker ⛔ 沒有被呼叫；
  正常的 `.env` 要呼叫 down 與 up、帶進去的密碼長度 64（只印長度）。對照組：舊的那一行在同樣五種情況下都要照樣部署（重現 review 的發現）。
* （v1.2）再加尾端空值的重複鍵、順序反過來、`export` 或行首空白的重複、`=` 前後有空白、只寫鍵名、引號、CRLF、`#`、`$`（都要 rc 1），
  與註解掉的舊密碼、檔尾沒有換行、有其他鍵（都要 rc 0）。每種都用 `docker compose -f docker-compose.redis.yml --env-file <檔> config` 交叉比對：
  `deploy.sh` 接受的，兩邊讀到的值要相同（只比 hash 前綴）。
* （v1.3）再加冒號賦值（接在正常那行後面、單獨一行、冒號後沒有空白）、行尾註解、行首有空白的註解、有其他鍵（都要 rc 1），
  與值用到全部允許的符號、有空行（都要 rc 0）；對照組是 v1.2 的寫法跑 review 的例子。
* `scripts/check-doc-refs.py` 問題 0；新增的行掃一次禁止標記有沒有接否定詞。

## 七、執行順序

1. 使用者確認本計畫（D1～D4）。
2. A、B、C 的檔案修改 → `config` 驗證。
3. dev 實跑（六）。
4. 文件（D）→ review → 使用者 commit。
5. review 通過後：現況已在文件裡，刪除本計畫書。

## 八、完成後的歸檔位置

* `docs/development-workflow.md`：「live 的 Redis：獨立 stack、密碼與 port」、dev port 只綁本機（含 v1.1 的 Docker 28.0.0 以前的已知限制）。
* `docs/architecture.md`、`docs/development-guide.md` 的部署段落。
* 各 compose 檔的檔頭註解（設計理由）。

## 十、實作紀錄

### 2026-11-03（⚠️ 待 review）

* A `docker-compose.redis.yml`：拿掉 `ports:`；`command` 經 `docker-entrypoint.sh`；`REDIS_PASSWORD`（`:?`）與 `REDISCLI_AUTH`；healthcheck 比對 `PONG`；補資源上限；檔頭寫明設計理由與 `.env` 的建法。
* B `docker-compose.yml`：backend 的 `REDIS_PASSWORD: ${REDIS_PASSWORD:-}`；`deploy.sh` 範本：`export REDIS_PASSWORD="$(sed … /path/to/redis-stack/.env)"`（只有寫法，⛔ 沒有值；review 第 1 輪改成 fail-closed，見下）。
* C `docker-compose.dev.yml`：四個 port 改成 `127.0.0.1:…`；dev Redis 加 `--requirepass`（`DEV_REDIS_PASSWORD`，預設 `dev-redis-only`）、經 entrypoint、healthcheck 比對 `PONG`；dev backend 帶同一個變數；檔頭註明只綁本機的理由。
* D 文件：`architecture.md`「生產 / Docker」改成三個 stack 的對照表（含 D4：repo 的 `docker-compose.postgres.yml` 是 pgtest，⛔ 不是正式的 postgres）；
  `development-guide.md` 同一段改寫並指向它，port 表的 Redis 註明⛔ 不發布到 host；
  `development-workflow.md`：「Docker 環境區隔」的 dev port 改成只綁本機、新增 port 規則與「live 的 Redis：獨立 stack、密碼與 port」；「2. 啟動 dev stack 做 smoke test」補 SSH tunnel 與 dev Redis 的密碼。
* 驗證：
  * `config`：Redis 範本⛔ 沒設 `REDIS_PASSWORD` 時報錯（`required variable REDIS_PASSWORD is missing a value`）、有設時通過且⛔ 沒有 port；
    `docker-compose.yml` 有設時帶入、沒設時是 `""`；dev 的四個 port 都是 `host_ip: 127.0.0.1`，Redis 與 backend 都是 `dev-redis-only`。`bash -n deploy.sh` 通過。
  * Redis 範本與 live 的 `/opt/stacks/scripts/redis/docker-compose.redis.yml`：去掉註解、排序之後完全相同。
  * dev 實跑（`NO_BUILD=1 scripts/smoke-dev.sh`，rc 0）：dev Redis 帶密碼 `PONG`、不帶被拒、以 uid 999 執行；dev backend `redis connected`；
    host 上的 15432／16379／18080／18001 都只綁 `127.0.0.1`；postgres、redis 的 healthcheck healthy。驗完 `down`（⛔ 沒有帶 `-v`），volume 保留。
* 附帶發現（⛔ 不在本計畫範圍，使用者決定）：live 的 postgres（`postgres-postgres-1`，`/opt/stacks/scripts/postgres/`）發布在 `0.0.0.0:5432`。

### 2026-11-03 review 第 1 輪（兩項 P2，✅ 使用者確認修法；⚠️ 修正待 review）

* **P2-1 `deploy.sh` 讀不到 Redis 密碼仍會繼續部署**：`export X="$(sed …)"` 吞掉 sed 的結束碼；檔案在但沒有這個鍵時 sed 本來就回 0。
  * 修正：分開讀值、檢查結束碼，再檢查值是一行非空值（空值與鍵重複都擋），通過才 `export`；失敗時在 `down` 之前 `exit 1`，錯誤訊息只印路徑。
  * 驗證（假 `docker`，見六）：舊的那一行在檔案不存在、沒有讀取權限、空檔、沒有這個鍵、鍵重複時都 rc 0 且照樣呼叫 down／up（密碼長度 0，鍵重複時是 129）——重現 review 的發現；
    新的寫法這五種都 rc 1、docker ⛔ 沒有被呼叫，正常的 `.env` rc 0、down／up 都帶到長度 64 的密碼。`bash -n` 通過。測試用的 `.env` 驗完已刪。
  * ⚠️ live 的 `deploy.sh` 若也是舊的那一行，要由使用者換成同一段（Claude ⛔ 不讀 live 的 `deploy.sh`）。
* **P2-2 缺少 Docker 28.0.0 以前的 loopback 限制**：官方說明這些版本裡，同一個 L2 網段的其他主機連得到發布在 localhost 的 port
  （[Publishing ports](https://docs.docker.com/engine/network/port-publishing/#publishing-ports)）。所以上面「只綁 `127.0.0.1`」的 `ss` 結果⛔ 不能當成外部連不到的證據。
  * 唯讀查到的事實：Engine 是 `20.10.5+dfsg1`；對外只有 `venet0`（點對點、`NOARP`，預設路由走它），其餘是 docker 的 bridge 與 veth。
    網卡類型會影響鄰居能不能把這種封包送進來，但⛔ 沒有實測，⛔ 不能據此判定安全。
  * 影響範圍：dev 的四個 port，以及 pgtest、MySQL 測試起著的時候。live 的 `docker-compose.yml` 與 Redis 都⛔ 沒有發布 port，不受影響。
  * 修正：`development-workflow.md`「Docker 環境區隔」補一則已知限制；「2. 啟動 dev stack 做 smoke test」的 SSH tunnel 說明指向它；
    「live 的 Redis」的確認方法加上 `docker port redis-redis-1`（沒有輸出＝沒有發布任何 port），⛔ 不只靠 `ss`。
  * 待裁決（這次⛔ 沒有做）：補強方式——升級 Docker 到 28.0.0 以上，或自己在 iptables 加規則，丟棄從非 `lo` 進來、目的地是 `127.0.0.0/8` 的封包。

### 2026-11-03 review 第 2 輪（一項 P2，✅ 使用者確認修法；⚠️ 修正待 review）

* **P2 尾端空值的重複鍵會繞過第 1 輪的檢查**：`REDIS_PASSWORD=<值>` 後面再接 `REDIS_PASSWORD=` 時，sed 印出「值＋換行＋空行」，
  `$(…)` 吃掉結尾的換行後只剩那個值，「一行非空值」的檢查通過；compose 讀同一份檔卻是空值、拒絕解析。
  同一類的還有：`export` 開頭或行首有空白的第二行 sed 看不到、引號與 `\r` compose 會去掉、`$` compose 會展開——都讓 backend 帶的密碼和 Redis 的不同。
  * 修正：改用 awk 解析原始檔——compose 可能當成這個鍵的行要剛好一行，而且那一行要是 `REDIS_PASSWORD=<值>`、值只用英數與 `._+/=-`；
    格式不對（awk 結束碼 3）與讀不到（其他結束碼）分開報錯，都在 `down` 之前 `exit 1`。`docker-compose.redis.yml` 檔頭補上 `.env` 的格式；
    `development-workflow.md`「live 的 Redis」補格式要求與「為什麼檢查原始檔」。
  * 驗證（假 `docker`，見六；這台只有 gawk，⛔ 沒有 mawk 可測）：20 種情況。
    * 第 1 輪的寫法（對照組）：尾端空值的重複鍵 rc 0、照樣呼叫 down／up——重現 review 的發現；`export`／行首空白的重複、只寫鍵名、引號、CRLF、`#`、`$` 也都放行，
      其中引號、CRLF、`#`、`$` 帶進去的密碼長度是 65～66。
    * 新的寫法：16 種異常都 rc 1、docker ⛔ 沒有被呼叫；正常、註解掉的舊密碼、檔尾沒有換行、有其他鍵這 4 種 rc 0，down／up 都帶到長度 64 的密碼。
      路徑是目錄時 gawk 只警告、跳過，走到「格式不對」那條訊息——仍是 rc 1，只是訊息不是「讀不到」。
    * compose 交叉比對（Compose v5.1.3，`config` 只解析、⛔ 沒有起 container）：新寫法接受的 4 種，兩邊讀到的值相同。
      compose 本身：尾端空值的重複鍵、空檔、沒有這個鍵時報錯；同一個鍵寫兩次取後面那行（`export`／行首空白的第二行也算）；
      引號與 `\r` 會去掉；`$x` 會展開成空字串；`=` 前後的空白會去掉；值裡的 `#x` 保留。
    * `bash -n` 通過；測試用的 `.env` 驗完已刪。
  * ⚠️ live 的 `deploy.sh` 要換成這一段新的（第 1 輪的寫法也有這個漏洞）；Claude ⛔ 不讀 live 的 `deploy.sh`。（第 3 輪又改了，以第 3 輪為準。）

### 2026-11-03 review 第 3 輪（一項，✅ 使用者確認修法；⚠️ 修正待 review）

* **漏掉 compose 接受的冒號賦值**：`REDIS_PASSWORD=first` 後面接 `REDIS_PASSWORD: second` 時，第 2 輪的 awk 不認得第二行、忽略它，帶入 first；
  compose 卻採用 second。根本原因是「只計認得的寫法、其他行忽略」——compose 的 `.env` 語法認不完，這類漏洞會一直出現。
  * 修正（review 建議的第一個做法）：整份 `.env` 走白名單——只允許空行、從第一個字元就是 `#` 的註解、剛好一行 `REDIS_PASSWORD=<值>`（值只用英數與 `._+/=-`），
    其他任何一行都擋下（有其他鍵也擋，原本放行）。符合「`.env` 只存密碼」的約定。範本檔頭與 `development-workflow.md` 同步改寫。
  * ⛔ 沒有選「共用 compose 的解析結果」：`deploy.sh` 要另外知道 Redis compose 檔的路徑、用 python 解析 `config` 的 JSON、
    排除 shell 裡已 export 的 `REDIS_PASSWORD`（compose 讓 shell 環境優先於 `.env`），密碼也會經過整份 `config` 的輸出；每一處都是新的出錯點。
  * 驗證（假 `docker`＋compose 交叉比對，27 種情況，gawk）：
    * 第 2 輪的寫法（對照組）：review 的例子 rc 0、照樣 down／up，帶入的密碼和 compose 讀到的不同——重現 review 的發現。
    * 新的寫法：22 種異常都 rc 1、docker ⛔ 沒有被呼叫；正常、註解掉的舊密碼、檔尾沒有換行、值用到全部允許的符號、有空行這 5 種 rc 0，兩邊讀到的值相同。
    * compose 本身（Compose v5.1.3）：`KEY: value` 與 `KEY:value` 都算賦值；行尾的 ` # x` 會去掉；行首有空白的註解、其他鍵都照常解析。
    * `bash -n` 通過；Redis 範本只改註解，去掉註解後仍與 live 完全相同；測試用的 `.env` 驗完已刪。
  * ⚠️ live 的 `deploy.sh` 要換成這一段（第 1、2 輪的寫法都有漏洞）；live 的 `.env` 若不只那一行，deploy 會在 `down` 之前停下。

## 九、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-03 | 初版 |
| v1.1 | 2026-11-03 | review 第 1 輪：`deploy.sh` 讀密碼改成 fail-closed（P2-1）；記錄 Docker 28.0.0 以前只綁 `127.0.0.1` 的限制（P2-2） |
| v1.2 | 2026-11-03 | review 第 2 輪：`deploy.sh` 改成解析原始檔（鍵剛好一行、值的字元限制），補 compose 交叉比對；`.env` 格式寫進範本檔頭與文件 |
| v1.3 | 2026-11-03 | review 第 3 輪：`.env` 整份走白名單（取代 v1.2 的計數法），擋下冒號賦值等 compose 也接受的其他寫法 |
