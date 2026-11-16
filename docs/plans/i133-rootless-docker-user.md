# I-133：rootless Docker 下測試腳本的 `--user` 寫不進 cache（計畫書 v1）

> 狀態：v1 草稿，待使用者確認（2026-11-16）。⛔ 確認前不改腳本、不動 cache 目錄的權限。
> 對應：`docs/issue.md` I-133。跟 `todo.md` T-083 的關係：T-083 的驗證要跑 `python/scripts/test.sh`，所以本計畫先做。

## 一、目標與不做的範圍

**目標**

1. 在這台 rootless Docker 的主機（srv2141876，dev 與 live 共用）上，所有用 `--user` 跑 container 的腳本⛔ 不需要手動改權限就能跑。
2. rootful Docker 維持現在的行為（`--user` 本機 uid/gid），產出的檔案⛔ 不會變成 root 所有。
3. 把 2026-11-13 為了繞過而改成 777 的兩個 cache 目錄改回原狀，cache 裡被 subuid 佔有的檔案改回 dev 所有。
4. 以後新增的腳本⛔ 不能再直接寫 `--user "$(id -u):$(id -g)"`（由測試擋）。

**不做**

* vet 在 `MEM=700m` 被 OOM：由 `todo.md` T-079 承接。
* ⛔ 不動 live（live 的 compose 服務⛔ 不用 `--user`）；⛔ 不改 Docker daemon 的設定（例如 `userns-remap`）。
* ⛔ 不改各腳本的其他行為（記憶體、掛載、網路）。

## 二、現況（2026-11-16 查證）

* `docker info -f '{{.SecurityOptions}}'` 回 `[name=seccomp,profile=builtin name=rootless name=cgroupns]`。
* dev 帳號是 uid 1001；`/etc/subuid` 是 `dev:165536:65536`。rootless 模式下 container 的 uid 0 對應主機的 1001（dev），container 的 uid 1001 對應主機的 165536+1001−1 = **166536**。
* 所以 `--user 1001:1001` 在 container 裡寫出的檔案，在主機上是 166536 所有：
  * `~/.cache/stock_trading/gocache` 底下的子目錄是 `166536:166536`；`gomodcache` 底下的 `github.com`、`golang.org`、`go.uber.org` 也是。
  * 兩個 cache 目錄本身是 `drwxrwxrwx dev:dev`（11-13 的繞過）。
  * repo 裡⛔ 沒有 dev 以外的檔案（`find . ! -user dev` 是 0 筆）。
* 用 `--user "$(id -u):$(id -g)"` 的腳本共 15 支：
  * 測試：`backend/scripts/test.sh`、`python/scripts/test.sh`、`frontend/scripts/test.sh`、`scripts/test-redis-emission.sh`、`scripts/test-postgres-migrations.sh`、`scripts/test-mysql-migrations.sh`、`scripts/verify-event-timeline.sh`。
  * 評估與 replay 工具：`scripts/run-evaluation.sh`、`scripts/build-selection-report.sh`、`scripts/verify-regression-baseline.sh`、`scripts/smoke-replay-offline.sh`（兩處）、`scripts/run-replay-offline.sh`、`scripts/compare-replay-crossday.sh`、`scripts/compare-i129-replay.sh`、`scripts/finalize-evidence.sh`。
  * 它們都把 repo 或輸出目錄以讀寫掛進 container（例如 `python/scripts/test.sh` 掛 `python/` 到 `/app`，replay 工具寫 `$OUT_DIR`）。rootless 下 166536 寫不進 dev 擁有、權限 755 的目錄，所以不只 cache，**任何要寫進掛載目錄的腳本都會失敗**；只讀的掛載不受影響。
* `docs/development-workflow.md` 的「`--user`：以本機 uid/gid 執行，container 產出的檔案不會是 root 所有」是 rootful 的說法，rootless 下不成立。

## 三、做法

### 3.1 共用的判斷：`scripts/lib/docker-user.sh`（新增）

* 提供一個函式，依 Docker 的模式設定一個陣列（例如 `DOCKER_USER_ARGS`）：
  * rootless（`docker info` 的 `SecurityOptions` 含 `name=rootless`）→ **空陣列**：⛔ 不帶 `--user`，container 以 uid 0 執行，對應到主機的 dev，產出的檔案是 dev 所有。
  * rootful → `--user "$(id -u):$(id -g)"`（現況）。
  * `docker info` 失敗 → 照 rootful 處理（現況的行為，⛔ 不靜默改成 root）；之後的 `docker run` 本來就會因為連不到 daemon 而失敗。
* 覆寫用的環境變數（例如 `DOCKER_USER_MODE=auto|user|none`，預設 `auto`），只給測試與除錯用。
* 一個腳本只呼叫一次 `docker info`（結果存在變數裡）。
* 空陣列用 `${A[@]+"${A[@]}"}` 展開（有用 `set -u` 的腳本直接寫 `"${A[@]}"` 在舊版 bash 會報錯；實作時逐支確認）。

### 3.2 15 支腳本改用共用判斷

* 每支 `source` 這個 lib，把 `--user "$(id -u):$(id -g)"` 換成陣列展開；其他參數⛔ 不動。
* `-e HOME=/tmp` 保留（rootless 的 uid 0 也一樣用 `/tmp` 當 HOME，避免寫進 image 的 `/root`）。

### 3.3 防止再犯

* 新增 `scripts/test-docker-user.sh`：
  * 用假的 `docker` 驗證三種情況（rootless → 沒有 `--user`；rootful → `--user` 本機 uid/gid；`docker info` 失敗 → `--user`），以及覆寫變數。
  * 守門：repo 的 `*.sh` 裡⛔ 不能再出現直接寫的 `--user "$(id -u)`（lib 自己除外）。
* 掛進 `python/scripts/test.sh` 最前面那段「不需要 image、最快失敗」的檢查（跟 `test-doc-refs.sh` 同一段），所以每次跑 pytest 都會順便跑。

### 3.4 cache 目錄還原（Claude 在 dev 執行；⛔ 不碰 live）

1. 先盤點：`find ~/.cache/stock_trading ! -user dev` 列出被 subuid 佔有的路徑（只列路徑）。
2. 用 container 改回擁有者：rootless 下 container 的 root 就是主機的 dev，所以用一個不連網路的 container 對這兩個 cache 目錄 `chown -R 0:0`（主機上就是 dev）。⛔ 不刪 cache（刪了要冷 build，在這台主機上很慢）。
3. 兩個 cache 目錄改回 `755`（跟 `~/.cache/stock_trading` 底下其他目錄一致）。
4. 再跑一次步驟 1，結果要是 0 筆。

⚠️ 步驟 2 的 container 是一次性的維護操作（改檔案擁有者），⛔ 不是驗證；驗證一律用 repo 的腳本（第六節）。

## 四、受影響的檔案與資料流

* 新增：`scripts/lib/docker-user.sh`、`scripts/test-docker-user.sh`。
* 修改：上面 15 支腳本、`python/scripts/test.sh`（另外掛上新測試）、`docs/development-workflow.md`（`--user` 的說明）。
* 資料流：⛔ 沒有變化。只改 container 以哪個 uid 執行；rootful 主機上的行為不變。
* ⛔ 不影響 live 的任何 runtime（live 的服務由 compose 啟動，⛔ 不經過這些腳本）。

## 五、風險與回滾

| 風險 | 處理 |
|---|---|
| rootless 下 container 以 root 執行 | root 只在 user namespace 裡，對主機來說就是 dev，權限⛔ 沒有變大。 |
| 有 image 刻意用非 root 的 `USER` | 實作時逐一確認用到的 image（golang、python、node、本專案的 python image）的預設使用者；之後新增 image 時也要看。 |
| rootful 的行為被改壞 | 這台主機沒有 rootful 環境可以實測，只能用假 `docker` 驗證判斷（見第六節）；判斷失敗時退回 `--user`，等於現況。 |
| replay／評估工具沒有實跑 | 那幾支跑一次要很久、會吃大量記憶體；只做靜態檢查與守門，並在完成報告裡列出「沒有實跑」的清單。 |
| cache 還原改錯 | 只動兩個 cache 目錄；真的壞了可以整個刪掉重建（只是慢）。 |

**回滾**（舊腳本帶 `--user 1001:1001`，在主機上是 166536；第 3.4 節已經把 cache 遞迴改成 dev 所有、頂層 755，只改回頂層的 777 ⛔ 不夠——內層目錄是 755 的 dev 所有，166536 仍然寫不進去）：

1. `git revert` 這次的 commit。
2. 用不連網路的 container 對兩個 cache 目錄 `chown -R 1001:1001`（container 的 uid 1001 就是主機的 166536，跟舊腳本寫出來的擁有者相同），頂層再改回 `777`。⛔ 不刪 cache。
3. 驗證：`find ~/.cache/stock_trading/gocache ~/.cache/stock_trading/gomodcache -mindepth 1 -user dev` 是 0 筆；用舊腳本跑一次 `backend/scripts/test.sh`，⛔ 沒有 `permission denied`，而且第二次跑時套件顯示 `(cached)`（cache 有被重用）。

這套回滾在實作時演練一次（第六節第 5 點），⛔ 不是只寫在紙上。

## 六、測試與驗證

1. `scripts/test-docker-user.sh` 全部通過；故意把 lib 改成「永遠不帶 `--user`」或「永遠帶 `--user`」的突變，各自要讓對應的案例失敗。
2. `bash -n` 與 shellcheck（docker image `koalaman/shellcheck:v0.10.0`）檢查 15 支腳本與 lib。
3. 在這台主機先做第 3.4 節的 cache 還原，再實跑（記憶體照現況：Go 測試用 `MEM=1500m`，見 T-079）：
   * `backend/scripts/test.sh`
   * `python/scripts/test.sh`（含新測試）
   * `frontend/scripts/test.sh`
   * `scripts/test-postgres-migrations.sh`
   * 每支跑完確認 repo 與 cache 裡⛔ 沒有 dev 以外的檔案（`find ! -user dev` 是 0 筆）。
4. 其他 11 支：靜態檢查加守門，列在完成報告的「沒有實跑」。
5. **回滾演練**（review 2026-11-16 要求）：在第 3 點通過之後，
   1. 用 `git worktree` 取出改之前的版本（⛔ 不動工作目錄），照第五節的回滾步驟 2 改回 cache 的擁有者與權限；
   2. 用舊版的 `backend/scripts/test.sh` 跑兩次：⛔ 沒有 `permission denied`，第二次的套件顯示 `(cached)`；
   3. 再照第 3.4 節把 cache 改回 dev 所有、頂層 755，用新版的腳本跑一次確認仍然正常，最後移除 worktree。
   結果（含兩次的 `(cached)` 套件數）寫進實作備註。

## 七、完成後要歸檔的位置

* `docs/development-workflow.md`：把 `--user` 那段改成「依 Docker 模式決定」的現況說明（rootless 與 rootful 各自的行為、`DOCKER_USER_MODE`、cache 目錄的權限）。
* `scripts/lib/docker-user.sh` 的檔頭註解寫明原因。
* `docs/issue.md` I-133 改成「已實作／待 review」；review 確認後收斂。

## 八、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-16 | 初版。 |
| v1.1 | 2026-11-16 | review [P2]：回滾只改回頂層 777 不夠（內層已遞迴改成 dev 所有、755，舊腳本的 subuid 仍寫不進去）→ 回滾補上「擁有者改回 166536」的步驟與驗證（`(cached)` 證明 cache 被重用），並在實作時演練一次（第六節第 5 點）。 |
