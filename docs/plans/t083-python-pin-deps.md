# T-083：鎖定 python 的套件版本與基底 image，讓 python image 可以重現（計畫書 v1）

> 狀態：v1.2 已確認；repo 與 dev 的部分已實作（2026-11-16），待 review；**live 的部署（3.4、第六節第 5 點）還沒做**。實作備註在「八之前：實作備註」。
> 對應：`docs/todo.md` T-083。相關：`docs/issue.md` I-116（本計畫做的是它「待決策」表的第一項）、I-133（本計畫的驗證要跑 `python/scripts/test.sh`，I-133 先做）。

## 一、目標與不做的範圍

**目標**

1. `python/requirements.txt` 改成**完整的精確釘版**：直接依賴與間接依賴都寫 `==`，版本**以 live 現在跑的組合為準**。
2. `python/Dockerfile` 的基底 image 固定到 digest。
3. 驗收：照新檔 build 出來的 image，`pip freeze --all` 跟 live 現在的完全相同；dev 與 live 用同一份組合。
4. 之後升級套件變成一次明確的變更（改釘版 → build → 測試 → 部署），⛔ 不再因為快取失效而默默換版。

**不做**

* ⛔ 不升級任何套件（包括有新版的）；⛔ 不移除套件。
* ⛔ 不改 python 的程式碼。
* 部署時自動比對新舊 image 的 `pip freeze`（會再改到 `deploy.sh`）：使用者 2026-11-16 決定這次不做，列在第九節的後續。
* I-116 的其他選項（bundle 記錄訓練時的套件版本、保存 image tarball）⛔ 不在本計畫。
* ⛔ 不改 backend、frontend。

## 二、現況（2026-11-16 查證）

**live 的 python（使用者在 live 執行，唯讀）**

* Python 3.11.18、pip 24.0；Debian 13.7；`libgomp1` 14.2.0-19（Dockerfile 用 apt 裝的系統套件）。
* python-server 與 python-worker 的 `pip freeze --all` 相同（排序後的 md5：⚠️ 跟排序的 locale 有關——live 上在主機排序（en_US.UTF-8）是 `1c801990173f9d2b7b0fd632ec9a7655`，`LC_ALL=C sort` 是 **`39c5d9d45667ea4d872b9e746ac99600`**；之後一律用 `LC_ALL=C` 的值，見實作備註），共 51 個套件。這份清單⛔ 不含機密，原文保存在第十節的附錄 A。

**repo**

* `requirements.txt` 有 15 個直接依賴：只有 `backtrader` 寫 `==`，其他 14 個是 `>=` 下限（例如 `pandas>=2.0.0`、`scikit-learn>=1.4.0`）；間接依賴（例如 `scipy`、`numba`、`starlette`）完全沒列。
* `Dockerfile` 是 `FROM python:3.11-slim`（浮動的 tag），`pip install --no-cache-dir -r requirements.txt`。
* 用到 `requirements.txt` 的地方：`python/Dockerfile`（dev 與 live 的 image、`python/scripts/test.sh` 的測試 image 都由它 build）、`python/setup.sh`（本機 venv）。

**dev 與 live 的差異**：dev 現在的 python image（`stock_trading_dev-python-server`）跟 live 比，只差 **`fastapi`：dev 0.163.1、live 0.162.5**，其他 50 個相同，Debian 與 `libgomp1` 也相同。也就是說，dev 的 pytest 驗的組合跟 live 差一個套件。

**為什麼要做**

* 2026-11-16 部署時，live 兩個 python container 正在跑的 image 已經不在 image 庫，⛔ 沒有可以完整退回的版本；而 `>=` 的釘法讓「重新 build 一份一樣的」也做不到。
* I-116：凍結 bundle 的模型檔（pickle）跨 sklearn 版本可能給出不同結果；`requirements.txt` 的下限釘法讓 image 一消失就無法重建同一個環境。

## 三、做法

### 3.1 `requirements.txt`

* 用附錄 A 的清單，扣掉 `pip`、`setuptools`、`wheel`（它們來自基底 image，由 3.2 的 digest 固定；`pip install -r` 去改它們反而會動到 pip 本身），共 48 個，全部寫 `==`。
* 分成兩段：
  * **直接依賴**（現在的 15 個）：版本改成 live 的值，保留原本的註解（用途）。`uvicorn[standard]` 保留 extras，寫成 `uvicorn[standard]==0.54.0`。
  * **間接依賴**（其他 33 個）：一段一行，開頭註解說明「由直接依賴帶進來，版本照 live 的 `pip freeze`；升級時整份一起重新產生」。
* 檔頭註解寫明：版本來源（2026-11-16 live 的 `pip freeze --all`）、升級的程序（第七節）、⛔ 不能只改直接依賴（間接依賴會跟著漂）。
* 採用「直接改 `requirements.txt`」，⛔ 不另外產生 lock 檔（使用者 2026-11-16 決定：比較簡單）。

### 3.2 `Dockerfile`

* `FROM` 改成帶 digest 的寫法：`python:3.11.18-slim-trixie@sha256:…`（Python 3.11.18、Debian 13 trixie，跟 live 相同）。digest 在實作時取得：
  1. 先從 live 現在的 python image 的 layer，比對候選基底 image 的 layer（基底 image 的 layer 會是 live image 最前面那幾層）。比得上就用那個 digest，這是最確定的「跟 live 同一份基底」。
  2. 比不上（基底 image 已經更新過）就用實作當天的 digest，並且驗證 Python 3.11.18、pip 24.0、setuptools 79.0.1、wheel 0.46.3、Debian 13.7；有差異就停下回報，⛔ 不自己決定。
* `apt-get install libgomp1` ⛔ 不釘版本：Debian 的套件庫會移除舊版，釘了會讓之後的 build 直接失敗。改成在驗收時記錄版本（3.4、第六節），由人判斷。
* `pip install` 之後加 `pip check`（依賴的版本互相不衝突才算 build 成功）。

### 3.3 `setup.sh`（本機 venv）

* 指令不用改，但釘版之後本機也會裝同一組版本。⚠️ `development-guide.md` 提到 Windows 的 `.venv`：這些版本在 Windows 上有沒有 wheel ⛔ 沒有驗證，寫進已知限制。

### 3.4 dev 與 live 的換版

* **dev**：照新檔 build dev 的 python image（`scripts/smoke-dev.sh` 會 build），驗收見第六節。dev 只有 `fastapi` 會從 0.163.1 換回 0.162.5。
* **live**：用 `deploy.sh` 部署（它會 build 三個服務；backend 用快取、內容不變）。python 的 build 快取一定失效（`requirements.txt` 與 `FROM` 都改了），要從網路重新裝套件，build 時間會比平常長（scipy、numba、lightgbm、shap 都不小），所以：
  * 時段照 `development-workflow.md`「挑窗口」，預估要多留時間（用 13:56～14:40 那一段或 `daily_close` 之後，實際 build 時間在 dev 先量）。
  * 部署前先替兩個 python container **正在跑的 image** 打回滾標籤（`development-workflow.md`「部署腳本：失敗即停」的做法）。
  * 部署後比對新 container 的 `pip freeze --all`：`LC_ALL=C sort` 之後的 md5 要等於 `39c5d9d4…`；Debian 與 `libgomp1` 的版本照記錄。
  * 預期 live 的套件**完全不變**（版本就是從 live 抄來的），所以這次部署的實際效果是「換成一個可以重現的 image」。

### 3.5 新增 worker 的 dev 驗收腳本：`scripts/smoke-dev-worker.sh`

依 `AGENTS.md` 與 `development-workflow.md`：驗證要走 repo 的腳本，⛔ 不用手打的 compose／docker 指令當主要驗證路徑。`scripts/smoke-dev.sh` ⛔ 不支援 python-worker（`worker` profile、自己的 build 與 image），所以另外新增一支，⛔ 不改 `smoke-dev.sh` 的既有行為（它會先 `down`，語意不同）。

* 只用 `docker-compose.dev.yml` 的 dev project；跟 `smoke-dev.sh` 一樣擋 `COMPOSE_PROJECT_NAME`（不能被設定，連空字串都不行），⛔ 不會打到 live 的 project。（v1.4：compose 檔固定、⛔ 不接受覆寫，並在改變狀態之前驗證解析出來的 project 是 `stock_trading_dev`；每個 docker 指令都有上限；收尾失敗回傳非 0。v1.5：逾時加 `-k`（`KILL_AFTER`）、image 只認 `stock_trading_dev-python-worker`——見實作備註的 review 修正。）
* 流程：
  1. **開始前就有 python-worker 的 container**（不管是不是在跑）→ 停下、⛔ 不動它（不是本次建立的，⛔ 不能被收尾清掉）。
  2. postgres 用 `up -d --wait postgres` 起到 healthy（已經在跑就不影響）。
  3. 查 dev DB 的 `backtest_jobs`：有 `status='pending'` 就停下、⛔ 不啟動 worker（worker 一起來就會執行它們）。
  4. `--profile worker build python-worker`（`NO_BUILD=1` 時跳過），記下 image ID。
  5. 選用的 `EXPECT_FREEZE_MD5`：用這個 image 跑 `pip check` 與 `pip freeze --all | sort | md5sum`（不連網路、⛔ 不啟動 worker 程式），不相符就停下。
  6. `--profile worker up -d --no-deps python-worker`；從這一步起註冊收尾。
  7. container 的 `.Image` 要等於第 4 步的 ID。
  8. 等 log 出現 `poll #N`（`POLLS`，預設 3），**上限 `WAIT_SECONDS`（預設 90 秒）**；期間每次確認 container 是 running、`RestartCount` 0。
  9. log 要有 `worker started`，⛔ 不能有 `Traceback`、`poll #… error`。
* **收尾**（`trap`，成功、失敗、逾時、Ctrl+C 都會執行）：只停止並移除**本次建立的** python-worker container（`--profile worker rm -sf python-worker`），⛔ 不動 postgres 等其他服務；失敗時先印出 `LOG_TAIL` 行 worker 的 log 再收尾。
* 結束碼：0 通過、非 0 失敗；輸出只有步驟、image ID、md5 與 log 摘要，⛔ 不印 DSN。
* 測試：新增 `scripts/test-smoke-dev-worker.sh`，用假的 `docker`：正常通過；有 pending 的任務 → ⛔ 沒有 `up`；開始前已經有 worker → ⛔ 沒有 `up`、⛔ 沒有 `rm`；image 不符、`RestartCount` 不是 0、出現 `Traceback`、等不到 `poll #3`（在上限內結束）、md5 不符 → 失敗而且**有收尾**；收尾只針對 python-worker。突變：拿掉 `trap` 要讓「失敗也有收尾」的案例失敗。測試⛔ 不需要真的 docker，掛進 `python/scripts/test.sh` 那段「不需要 image」的 shell 測試。

## 四、受影響的檔案、模組與資料流

* 修改：`python/requirements.txt`、`python/Dockerfile`、`python/scripts/test.sh`（掛上新測試）。
* 新增：`scripts/smoke-dev-worker.sh`、`scripts/test-smoke-dev-worker.sh`（驗證流程的修改，3.5）。
* 文件：`docs/development-workflow.md`（新增「python 的套件版本與升級」）、`docs/issue.md` I-116（「待決策」表第一項改成由本計畫處理）、`docs/todo.md` T-083 的狀態。
* runtime：dev 與 live 的 python-server、python-worker（兩個以上 runtime → 本計畫書）；`python/scripts/test.sh` 的測試 image。
* 資料流與 API：⛔ 沒有變化。live 的套件版本不變；dev 只換 `fastapi` 一個。
* ⚠️ 交易訊號與 SR 決策：live 的套件不變，所以⛔ 不影響；dev 的 `fastapi` 只跟 HTTP 層有關，⛔ 不碰 SR 的計算套件（numpy、pandas、scikit-learn、lightgbm 都已經相同）。

## 五、主要風險與回滾

| 風險 | 處理 |
|---|---|
| 這些版本在套件庫上被撤下（yanked）或拿不到 | build 時就會失敗、⛔ 不會裝到別的版本；dev 先 build 驗證，失敗就停下回報。 |
| 基底 image 的 digest 對不回 live 那份 | 照 3.2 第 2 點：驗證版本，有差異停下回報。 |
| `libgomp1` 在套件庫更新了 | ⛔ 不釘版本；驗收時記錄，跟 live 不同就停下回報（只影響 lightgbm 的 OpenMP runtime）。 |
| live 的 python build 時間太長，撞到排程 | 在 dev 先量 build 時間，照實際時間挑窗口；`deploy.sh` 在 `down` 之前失敗都不會停服務。 |
| 釘死之後，安全性更新不會自動進來 | 這是刻意的取捨：升級改成明確的變更（第七節）；寫進 `development-workflow.md`。 |
| Windows 本機 venv 裝不起來 | 已知限制，寫進文件；⛔ 不為了 Windows 改版本。 |

**回滾**

* repo：`git revert`。
* live：部署前打的回滾標籤把 python 兩個服務的 `latest` 標回去，再 `compose up -d --no-build`，並分別確認三個服務（`development-workflow.md`「部署腳本：失敗即停」）。

## 六、測試與驗證

1. **dev build**：照新檔 build python image：
   * `pip check` 通過；
   * `pip freeze --all | LC_ALL=C sort | md5sum` 等於 live 的 `39c5d9d45667ea4d872b9e746ac99600`（不等就列出差異、停下）；
   * 記錄 Python、pip、Debian、`libgomp1` 的版本，跟第二節的 live 值比對；
   * 記錄冷 build 的時間與 MemAvailable 的低點（給 live 挑窗口用）。
2. **pytest**：`python/scripts/test.sh` 全部通過（I-133 修好之後；它會用同一份 Dockerfile build 測試 image）。
3. **dev smoke**：`scripts/smoke-dev.sh` 起 dev stack（backend、python-server，加上 postgres、redis），python-server 的 `/health` 正常。
4. **dev 的 python-worker**：`EXPECT_FREEZE_MD5=39c5d9d45667ea4d872b9e746ac99600 scripts/smoke-dev-worker.sh`（3.5），在第 3 點的 dev stack 起好之後跑；通過條件就是腳本的結束碼 0（image 是本次 build 的、`pip check` 與 md5 相符、持續運行到 `poll #3`、啟動 log 沒有錯誤），結束後 worker 已被腳本移除。另外先跑 `scripts/test-smoke-dev-worker.sh`。
5. **live**（使用者執行，Claude 給指令和預期輸出）：部署前後的 `pip freeze --all | LC_ALL=C sort | md5sum` 都是 `39c5d9d4…`；三個服務的確認照「部署腳本：失敗即停」；部署後第一個用到 python 的排程（例如 17:00 的 `sr_analysis`）是 `success`。

## 七、之後升級套件的程序（寫進 `development-workflow.md`）

1. 在 dev 改釘版（一次只升級一組相關的套件），build、`pip check`、跑 `python/scripts/test.sh`。
2. 影響 SR 計算的套件（numpy、pandas、scikit-learn、lightgbm、shap、scipy）升級時，還要做 SR 相關的回歸驗證（實作時對照 `sr-zone-scoring.md` 現有的驗證方式訂出範圍），並注意 I-116（模型檔的 pickle 跨版本）。
3. 用新的 `pip freeze --all` 重新產生間接依賴的那一段，⛔ 不手改。
4. 部署照「部署腳本：失敗即停」，部署前後比對 `pip freeze`。

## 八之前：實作備註（2026-11-16；已改好、未 commit，待 review）

**改了什麼**

* `python/requirements.txt`：48 個套件全部 `==`（直接依賴 15 個保留原註解；間接依賴 33 個一段；`pip`／`setuptools`／`wheel` 不列）。版本與附錄 A 逐一相同。
* `python/Dockerfile`：`FROM python:3.11.18-slim-trixie@sha256:07d4824036af339fa1b6ef769cc99d9c10a141769538b52b3b597c84f7a7b535`（2026-11-05 建立；Python 3.11.18、Debian 13.7、pip 24.0、setuptools 79.0.1、wheel 0.46.3、`packaging` 26.3，都跟 live 相同）；`pip install` 之後 `pip check`；`libgomp1` 不釘版本的理由寫在註解。
* 新增 `scripts/smoke-dev-worker.sh`、`scripts/test-smoke-dev-worker.sh`（3.5），後者掛在 `python/scripts/test.sh` 的「不需要 image」那一段。
* `docs/development-workflow.md`：新增「python 的套件版本與升級（T-083）」；smoke 那一節補上 `smoke-dev-worker.sh`。

**基底 image 的 digest（3.2）**：實作當天的 `python:3.11.18-slim-trixie` 有 4 層 layer，跟 dev 現有 python image 的前 4 層**完全相同**（4 層 layer 清單的 sha256 前 12 碼都是 `2d613846a388`）。live 的那一份還沒比——要在 live 上跑一行唯讀指令（給使用者的指令裡有），在部署前比完。

**驗證**

* `scripts/test-smoke-dev-worker.sh` 25 項通過。突變：拿掉 `trap cleanup EXIT` → 7 項失敗（失敗與被 TERM 時都沒有收尾）；拿掉 pending 的檢查 → 1 項失敗。shellcheck 0 則。
* `python/scripts/test.sh -q`（第一次用釘版的 Dockerfile 冷 build）：rc 0、219 秒；`pip install` 42 秒、export 36 秒；pytest 1915 passed、1 skipped；`test-docker-user.sh` 19 項、`test-smoke-dev-worker.sh` 25 項通過。整段的 MemAvailable 低點 4840 MiB（開始 5253 MiB）。
* 測試 image：Python 3.11.18、pip 24.0、Debian 13.7、`libgomp1` 14.2.0-19、`pip check` 通過；`pip freeze --all` 跟附錄 A **逐行相同**（`diff` 0 行）。
* `scripts/smoke-dev.sh`：rc 0、164 秒（backend 與 python-server 的 `/health` 正常）；dev 的 python-server image 的 md5 也是 `39c5d9d4…`、`libgomp1` 14.2.0-19。
* `EXPECT_FREEZE_MD5=39c5d9d45667ea4d872b9e746ac99600 scripts/smoke-dev-worker.sh`：rc 0、40 秒；pending 0 筆、`pip check` 通過、md5 相符、container 用的是本次的 image（`658d1c26`）、持續運行到 `poll #3`、⛔ 沒有 Traceback；結束後 worker 已被移除，dev stack 之後用 `compose down` 停掉（⛔ 不帶 `-v`）。
* dev 的 `fastapi` 從 0.163.1 換回 0.162.5（live 的版本）；pytest 與 smoke 都通過。
* python 的 replay shell 測試又留下 4 個 worktree（I-118），已清掉。

**review 修正（2026-11-16，`smoke-dev-worker.sh` 的 3 個 P2）**

1. **只能操作 dev project**：原本接受任意 `COMPOSE_FILE`、⛔ 沒有驗證 project（review 用 `name: stock_trading` 的假設定重現，仍會走到 `up`、`rm`）。改成 compose 檔固定是 `docker-compose.dev.yml`，並在任何會改變狀態的指令之前確認解析出來的 project 是 `stock_trading_dev`。
2. **收尾失敗要回傳非 0**：原本移除失敗只印警告、仍可能回傳 0。改成移除失敗、逾時或移除後仍查得到 → 結束碼 1（原本就失敗的保留原本的結束碼）。
3. **等待上限要涵蓋卡住的 docker 指令**：原本等待期間的 `inspect`／`logs` 沒有逾時（review 重現：`WAIT_SECONDS=1`，3 秒後仍未結束）。改成每個 docker 指令都用 `timeout` 包住（`CALL_TIMEOUT`、`BUILD_TIMEOUT`），等待期間單次查詢的上限再被剩餘時間截短；收尾整段共用 `CLEANUP_TIMEOUT`（印 log 最多用一半）。

* 測試從 25 項增加到 43 項：project 是 `stock_trading`／查不到／查詢卡住 → rc 2、⛔ 沒有 `up`／`rm`（project 不對時連 `ps` 都不跑）；`COMPOSE_FILE` 環境變數換不掉 dev 的 compose 檔；移除失敗、移除後仍查得到、收尾卡住 → rc 1；驗收失敗又移除失敗 → 仍是 rc 1；等待期間 `inspect`／`logs` 卡住（`WAIT_SECONDS=1`、`CALL_TIMEOUT=30`）→ 1～2 秒結束、rc 1、有收尾；啟動後確認 image 時卡住 → 2 秒結束、有收尾；啟動前 `ps`／`exec`／`image` 卡住 → 2 秒結束、⛔ 沒有啟動；新的參數驗證。
* 突變：拿掉 project 檢查 → 4 項失敗；收尾失敗不改結束碼 → 3 項失敗；等待期間的查詢不受剩餘時間限制 → 1 項失敗（卡 30 秒）。shellcheck 0 則。
* 測試本身的時序：假 docker 原本第 3 次讀 log 才出現 `poll #3`，離 `WAIT_SECONDS=3` 太近，偶爾在慢的時候誤判（10 次裡 1 次）；改成第 2 次讀就看得到，連跑 5 次都通過。
* dev 實跑（修正後）：`EXPECT_FREEZE_MD5=39c5d9d4… scripts/smoke-dev-worker.sh` rc 0、43 秒，worker 已移除；`COMPOSE_PROJECT_NAME=` → rc 2。之後用 `compose down` 停掉 dev（⛔ 不帶 `-v`）。

**review 修正（2026-11-18，`smoke-dev-worker.sh` 的逾時處理 P2）與 dev 實跑發現的問題**

1. **逾時要能砍掉忽略 SIGTERM 的程序**：原本 `timeout` 沒有 `-k`，逾時只送 SIGTERM（review 用忽略 SIGTERM 的假 docker 重現：`WAIT_SECONDS=1`、`CALL_TIMEOUT=1`，4 秒後仍未結束）。改成 `timeout -k "$KILL_AFTER"`（新的環境變數 `KILL_AFTER`，預設 5 秒；沒有 `--foreground`，所以 SIGKILL 送到整個 process group）。上限變成等待 `WAIT_SECONDS`＋`KILL_AFTER`、收尾 `CLEANUP_TIMEOUT`＋`KILL_AFTER`。
2. **取 image 名稱不能取第一行**（修正後 dev 實跑時發現）：`config --images python-worker` 會連 depends_on 的 postgres 一起列，順序不固定（連查 4 次有 1 次 postgres 在前）；`head -1` 取到 `postgres:16-alpine`，那次在 `pip check` 失敗、⛔ 沒有啟動 worker（fail closed，沒有誤判成通過），之前兩次實跑通過是剛好排在前面。改成只認 compose 預設的名稱 `stock_trading_dev-python-worker`（python-worker 沒有 `image:` 鍵；project 已在第 0 步驗過），而且要剛好出現一次。

* 測試從 43 項增加到 50 項：假 docker 的 `FAKE_IGNORE_TERM=1` 讓卡住的指令忽略 SIGTERM。等待期間 `inspect`／`logs` 卡住又忽略 SIGTERM（`WAIT_SECONDS=1`、`CALL_TIMEOUT=1`、`KILL_AFTER=1`）→ 4 秒內結束、rc 1、有收尾；收尾卡住又忽略 SIGTERM → rc 1；查 project 卡住又忽略 SIGTERM → rc 2、⛔ 沒有啟動；三種都確認卡住的程序已被砍掉、⛔ 沒有留成孤兒。`KILL_AFTER=0` → rc 2。假 docker 的 `config --images` 改成預設把 postgres 列在前面（另有 worker 在前、沒有 worker 兩種），postgres 的 image 給另一個 ID。
* 突變：拿掉 `-k` → 4 項失敗；取 image 改回 `head -1` → 8 項失敗。shellcheck 0 則。
* dev 實跑（修正後）連跑 3 次：rc 0，42／37／38 秒，每次都取到 `stock_trading_dev-python-worker`，worker 都已移除；之後用 `compose down` 停掉 dev（⛔ 不帶 `-v`）。
* `python/scripts/test.sh -q`：rc 0、201 秒；pytest 1915 passed、1 skipped；`test-docker-user.sh` 19 項、`test-smoke-dev-worker.sh` 50 項通過。

**跟計畫的差異**

* **md5 的基準改成 `LC_ALL=C` 排序**：計畫寫的 `1c801990…` 是 live 上 `docker exec … pip freeze --all | sort | md5sum`（`sort` 在主機上、locale en_US.UTF-8）算出來的；新 image 在 container 裡排序（C locale）得到 `39c5d9d4…`。兩份清單逐行相同，差的只是排序：en_US 不分大小寫，C 是大寫在前（`PyMySQL`、`PyYAML`、`Pygments`、`SQLAlchemy` 排到最前面）。之後一律用 `LC_ALL=C sort`，基準是 `39c5d9d45667ea4d872b9e746ac99600`；`smoke-dev-worker.sh` 也寫死 `LC_ALL=C`。計畫內文的 md5 已全部改成新的基準。
* 第 3.2 節的「先跟 live 的 layer 比」：dev 這邊比完了，live 的比對移到部署前（使用者執行）。

**還沒做**：live 的部署（3.4、第六節第 5 點），要另外排時段；部署前先比對 live 的基底 layer。

## 八、完成後要歸檔的位置

* `docs/development-workflow.md`：「python 的套件版本與升級」（釘版的原因、版本來源、升級程序、`libgomp1` 不釘的取捨、Windows venv 的限制）；smoke 那一節補上 `scripts/smoke-dev-worker.sh` 的用法、環境變數與收尾行為。
* `docs/issue.md` I-116：「待決策」表的第一項標成已由 T-083 處理，其餘選項維持待決策。
* `docs/todo.md` T-083 改成「已實作／待 review」；review 確認後收斂。

## 九、後續（⛔ 不在本計畫）

* 部署時自動比對新 python image 與正在跑的 container 的 `pip freeze`，不一致就停下等人確認（會改到 `deploy.sh`；使用者 2026-11-16 決定這次不做）。要做的時候另開 todo。

## 十、附錄 A：2026-11-16 live 的 `pip freeze --all`（python-server；python-worker 相同）

```text
annotated-doc==0.0.6
annotated-types==0.8.0
anyio==4.16.0
backtrader==1.9.78.123
certifi==2026.10.23
click==8.5.1
cloudpickle==3.1.2
fastapi==0.162.5
h11==0.16.0
httpcore==1.0.9
httptools==0.9.0
httpx==0.28.1
idna==3.20
iniconfig==2.3.1
joblib==1.6.0
lightgbm==4.7.0
llvmlite==0.50.0
narwhals==2.28.0
numba==0.68.1
numpy==2.4.6
opentelemetry-api==1.45.2
packaging==26.3
pandas==3.1.1
pip==24.0
pluggy==1.7.0
psycopg2-binary==2.9.13
pydantic==2.14.3
pydantic_core==2.50.4
Pygments==2.21.0
PyMySQL==1.2.4
pytest==9.2.0
python-dateutil==4.0.0
python-dotenv==1.2.4
PyYAML==6.0.3
scikit-learn==1.9.1
scipy==1.17.1
setuptools==79.0.1
shap==0.51.0
slicer==0.0.8
SQLAlchemy==2.1.8
starlette==1.10.0
threadpoolctl==3.8.0
tqdm==4.70.1
typing_extensions==4.16.0
typing-inspection==0.4.5
tzdata==2026.6
uvicorn==0.54.0
uvloop==0.23.0
watchfiles==1.3.0
websockets==17.4
wheel==0.46.3
```

## 十一、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-16 | 初版。 |
| v1.1 | 2026-11-16 | review [P2]：`smoke-dev.sh` 不會啟動 python-worker（`worker` profile、自己的 image）→ 第六節新增第 4 點：worker 的 build、確認用的是本次 image、持續運行到 `poll #3`、啟動 log 檢查，以及事先確認 dev 沒有 pending 的 `backtest_jobs`。 |
| v1.2 | 2026-11-16 | review [P2]：worker 驗收⛔ 不能用手打的 compose 指令當主要驗證（`AGENTS.md`）→ 新增 3.5 `scripts/smoke-dev-worker.sh`（等待上限、`trap` 收尾只移除本次的 worker、開始前已有 worker 就不動）與 fake docker 測試；第六節第 4 點改用這支腳本。 |
| v1.3 | 2026-11-16 | 實作（repo 與 dev）：md5 的基準改成 `LC_ALL=C` 排序（`39c5d9d4…`；原本的 `1c801990…` 是主機 en_US locale 的排序結果）；基底 image 的 layer 比對 dev 完成、live 移到部署前；新增實作備註。 |
| v1.4 | 2026-11-16 | review [P2]×3（`smoke-dev-worker.sh`）：只操作 dev project（compose 檔固定、驗證 project 名稱）、收尾失敗回傳非 0、每個 docker 指令都有上限（等待期間受剩餘時間限制、收尾共用一個上限）；測試 43 項。 |
| v1.5 | 2026-11-18 | review [P2]（`smoke-dev-worker.sh`）：`timeout` 加 `-k`（`KILL_AFTER`），忽略 SIGTERM 的 docker 也會被砍掉；dev 實跑發現 `config --images` 的順序不固定，改成只認 `stock_trading_dev-python-worker`；測試 50 項。 |
