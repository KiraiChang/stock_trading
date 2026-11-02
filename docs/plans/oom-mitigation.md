# 2 GiB host 的 OOM 改善 計畫 v4（2026-11-02，✅ 決定 D1～D6 依建議定案（2026-11-02）；v2 依 review 修正「二 B」2、3 與「二 D」3；v3 補「二 B」6 的實作細節；v4 補 `COMPOSE_PROJECT_NAME` 的守門；v5 補「二 D」1 的實作細節；v3～v5 ✅ 使用者 2026-11-02 確認）

需求（使用者 2026-11-02）：Claude 在 I-123～I-126 階段 B 的 dev 實測時被 OOM kill；同一階段的 review 也要求補跑 `RACE=1`，
而這台 host 的可用記憶體不夠跑。範圍（使用者確認）四塊都要涵蓋：讓 `RACE=1` 跑得動、dev 驗收流程、Claude session 本身、host 整體配置。

## 〇、唯讀盤點（2026-11-02）

| 項目 | 現況 |
|---|---|
| host | OpenVZ／Virtuozzo 容器（ploop 磁碟、kernel 4.19）。RAM 2048 MB；swap 512 MB 是虛擬的（`/proc/swaps` 顯示 `/dev/null`），白天常常用滿。`overcommit_memory=1`、`swappiness=10`。磁碟剩 8.2 GB |
| 白天的 MemAvailable | 約 530～680 MB（live 在跑、dev 停著） |
| 常駐的大戶（RSS） | dockerd 約 290 MB、live 的 postgres 約 180 MB、fin-api（dotnet，別的專案）約 100 MB、codex 兩支約 120 MB、tmux 約 90 MB、live 的 backend／python 三支合計約 100 MB |
| Claude | 單一 session 280～400 MB，隨 transcript 成長；`oom_score` 約 127（全機最高）、`oom_score_adj` 0——OOM 時第一順位就是它 |
| 這次的 OOM | 2026-11-02 10:2x，dev image build（Dockerfile 的 node stage `npm ci` ＋ Go 編譯）期間 Claude 被砍；背景的 build 照樣跑完。之前的實例：08-03、08-05、08-18、09-07、09-14（`development-workflow.md`「`MEM` 是上限，不是預留」等節） |
| `RACE=1` | `backend/scripts/test.sh` 檔頭的實測：race 版 `modernc.org/sqlite/lib` 單一 compile 峰值約 510 MB，`MEM` 要 700m 才跑得完，所以 MemAvailable 要 850 MB 以上。現況差 200～300 MB；race 用的 debian 版 `golang:1.25` image 也不在本機（只有 alpine 版） |
| dev 的 smoke | `scripts/smoke-dev.sh` 會先 `down`（⛔ 不帶 `-v`）再 `compose build`（backend 與 python-server 兩個 image）、`up -d`；沒有「只改環境變數、⛔ 不 build」的路徑，也沒有「只起需要的服務」的選項 |
| 可用的外部資源 | repo 有 GitHub remote，目前⛔ 沒有任何 workflow |

## 一、目標與不做的範圍

* **G1** 讓 `RACE=1 backend/scripts/test.sh ./internal/scheduler/...` 跑得完，補上 I-123～I-126 階段 B 的 review 要求（⛔ 不能拿 `-count=20` 代替）。
* **G2** dev 驗收流程不再把 Claude 擠掉：build 的峰值降低或避開、只改環境變數時⛔ 不 build、只起需要的服務。
* **G3** Claude session 的記憶體：重的步驟（image build、`RACE=1`、整包測試）有固定的做法，被砍之後接得回來。
* **G4** host 整體：OOM 真的發生時，先犧牲可以重來的東西（測試 container、dev stack），⛔ 不是 Claude 或 live；長期容量的選項列給使用者決定。

不做：
* ⛔ 不改 live 的程式、compose 與 `deploy.sh`；⛔ 不動 live 的 container 設定（`oom_score_adj` 只加在測試與 dev）。
* ⛔ 不改 kernel 參數、⛔ 不加 swap——OpenVZ 容器裡做不到（推測，⛔ 沒有實測；本計畫也⛔ 不去試）。
* ⛔ 不改 Dockerfile 的前端 stage（D5）；⛔ 不讓測試繞過 SQLite。
* 買更大的主機、搬走 dev／測試，只列在 D6 給使用者決定，⛔ 不在本計畫實作。

## 二、做法

### A. `RACE=1`（G1）

1. **不讓 Claude 跟它搶記憶體**（D1 建議 (a)）：由使用者在**結束 Claude 之後**自己的終端機執行，Claude 的 280～400 MB 就空出來，
   再加上 dev stack 停著，MemAvailable 估計可到 850～1,000 MB。Claude 先把指令寫成 scratchpad 外的暫存腳本（短行、輸出與 exit code 寫進
   repo 外的 log，例如 `~/.cache/stock_trading/race-logs/`），使用者執行完再開新的 session 讀 log。前例：I-074 ⑨-1 的「結束 Claude」步驟。
2. **時段**（D2）：避開 live 的批次——平日 17:05～22:00（晚間批次後移之後的空檔）或週末。
3. **事先拉 image**：`docker pull golang:1.25`（debian 版，約 800 MB 磁碟；拉的時候⛔ 不吃多少記憶體）。拉 image 要連外，執行前先問使用者。
4. **`backend/scripts/test.sh` 加 race 的前置檢查**：`RACE=1` 時 mem-guard 下修後的 `MEM` 低於 700m 就**直接中止**、印出還差多少，
   ⛔ 不讓它編十分鐘才被 cgroup 砍。設定 `RACE_MIN_MB`（預設 700）可覆寫。
5. **套件**：照 I-123～I-126 計畫書「八」，先跑 `./internal/scheduler/...`；`./internal/api/handler/`（409 的所有權測試）另跑一次。
   一次只跑一個套件（race 的 compile 結果會留在 repo 外的 `GOCACHE`，第二次會便宜很多）。
6. **detector 真的有效**（D3）：`development-workflow.md`「Race detector」要求先在有 race 的版本確認會報 `WARNING: DATA RACE`。
   做法：在 backend 的複本裡故意做出一個 race（實際的改法見「十」：單純拿掉一把鎖多半抓不到，只把寫入移到鎖外也不夠），跑同一個指令，確認報出 race；之後才跑正式版。多一次 race 執行。

### B. dev 驗收流程（G2）

1. **build 一次一個**：`smoke-dev.sh` 的 `compose build` 改成依序 `compose build backend`、`compose build python-server`。
   compose v2 預設會並行 build，兩個 image 的峰值會疊加（⚠️ 是否真的並行要實測確認：在 build 期間每 2 秒記一次 MemAvailable，比較低點）。
2. **`NO_BUILD=1`**：image 已經是現行程式碼、只改環境變數時，**只跳過 down 與 build**；依賴的啟動與就緒條件照下面 3 的規則，
   ⛔ 不因為跳過 build 就省掉。這次 dev 實測用的就是這條路（依賴本來就在跑、只重建 backend，幾秒、⛔ 不 build）。
3. **`SERVICES` 部分服務模式**：只起需要的服務，例如只驗排程時⛔ 不起 python-server（約 180 MB）。預設不變（全部起來、照 `depends_on`）。
   規則照 `docker-compose.dev.yml` 的依賴（backend 依賴 postgres healthy、redis healthy、python-server started；python-server 依賴 postgres healthy）：

   | 選了的服務 | 先起的依賴與就緒條件 | 最後怎麼起 |
   |---|---|---|
   | `backend`（沒選 `python-server`） | `compose up -d --wait --wait-timeout <WAIT_SECONDS> postgres redis`：等到兩者的 healthcheck 都是 healthy（`pg_isready`、`redis-cli ping`），逾時就中止並印診斷 | `compose up -d --no-deps backend` |
   | `python-server`（沒選 `backend`） | `compose up -d --wait --wait-timeout <WAIT_SECONDS> postgres` | `compose up -d --no-deps python-server` |
   | `backend python-server` 或預設（全部） | ⛔ 不另外處理：`compose up -d` 照 `depends_on` | — |

   * **`--no-deps` 只用在上一步確認依賴已經 healthy 之後**。dev 已經 down 時也成立：`--wait` 會把依賴起來並等到就緒；依賴本來就在跑時 `--wait` 立刻返回。
   * **健康檢查只看選了的服務**：backend 看 `BACKEND_URL`、python-server 看 `PYTHON_URL`；沒選的⛔ 不檢查（現在的腳本固定兩個都檢查，要改成依 `SERVICES`）。
     postgres、redis 的就緒由 `--wait` 判斷，⛔ 不另外打 HTTP。
   * **限制照實寫**：backend 的 `/health` ⛔ 不需要 python-server（2026-11-02 dev 實測：停掉 python-server 後重建 backend，`/health` 幾秒內 ok），
     但需要 python 的工作（例如 SR 分析）在這個模式下會失敗——只適合驗⛔ 不需要 python 的路徑。腳本在只選 backend 時印一行提醒。
   * `SERVICES` 只接受 `backend`、`python-server` 兩個名稱（postgres、redis 一律由規則帶起；python-worker 走 profile，⛔ 不在選項內）；其他值直接中止。
4. **override 檔**：腳本接受 `OVERRIDE=<檔案>`，轉成 `-f`，讓只為驗收用的設定留在 scratchpad、⛔ 不進 repo（目前要自己組 compose 指令）。
5. **文件**：`development-workflow.md`「2. 啟動 dev stack 做 smoke test」補上兩件事：
   * cron 要在**容器起來之後**才推算（這次在 build 之前算「+4 分鐘」，build 花了約 7 分鐘就錯過了）。
   * 判斷「image 是否已是現行程式碼」的做法（該文件已有 backend 比對建置時間的段落，沿用）。
6. **實作細節**（v3 補充；✅ 使用者 2026-11-02 確認）：
   * 部分服務模式**只 build 選了的服務**（只起 backend 就⛔ 不 build python-server）；兩個都選或預設時，依序 build 兩個。
   * `NO_BUILD=1` 時 `up` 一律加 `--no-build`：image 不存在就直接失敗。compose 遇到缺 image 的 build 服務會在 `up` 時自己 build，⛔ 不能讓它這樣補。
     `NO_BUILD=1` 也一併跳過 down，`SKIP_DOWN` 的值⛔ 不影響。
   * `OVERRIDE`：相對路徑以**呼叫時的目錄**解析成絕對路徑（腳本會 `cd` 到 repo 根目錄）；檔案不存在就在任何 docker 指令之前中止；只收一個檔。
     override 檔裡的相對路徑照 compose 的規則，以第一個 `-f`（repo 根目錄的 dev compose）為基準。
   * `SERVICES`：以空白或逗號分隔；重複的名稱忽略；空字串（或只有分隔符號）等於預設。
   * 診斷：等依賴逾時，印 postgres、redis 的 log；其他失敗印選了的服務的 log（現在固定印 backend 與 python-server）。
   * 開頭印一行模式摘要：選了的服務、`NO_BUILD`、override 檔。
   * **`COMPOSE_PROJECT_NAME` 有設定（含空字串）就中止**（v4；✅ 使用者 2026-11-02 確認）：compose 讓這個環境變數優先於 compose 檔的 `name:`
     （2026-11-02 以 `config --no-interpolate` 驗證：設成 `zz_probe` 時，dev compose 的 project 就變成 `zz_probe`），
     腳本的 `down --remove-orphans` 會被導到別的 project。改版前就有這個缺口，⛔ 不是 B1～B4 造成的。中止而⛔ 不是默默清掉：設了它通常代表 shell 正在操作別的 project。
     `COMPOSE_FILE` 是腳本文件寫明的覆寫，維持不變。

### C. Claude session（G3）

1. **重的步驟⛔ 不跟長 session 疊在一起**：image build、`RACE=1`、`backend/scripts/test.sh ./...` 之前，二選一——
   交給使用者在 Claude 結束後執行（A1 的做法），或先把進度寫進計畫書／scratchpad 再開新 session 執行。
2. **一律落地**：重的步驟一律 `> log 2>&1; echo $?`、放背景執行，Claude ⛔ 不靠前景的長等待拿結果。
   被砍之後的接手程序（先看 scratchpad 的 log、`docker ps`、`compose ps`，⛔ 不直接重跑）寫進 `development-workflow.md`——
   這次就是靠前一個 session 留下的 log 接回來的。
3. **長任務分段**：一個 session 只做一個階段（例如「review 修正」與「RACE」分兩個 session），transcript 小，RSS 就小。

### D. host 整體（G4）

1. **OOM 的犧牲順序**（D4）：測試 container 加 `--oom-score-adj=1000`（三支 `scripts/test.sh` 與它們共用的參數），
   dev compose 的每個服務加 `oom_score_adj: 500`。live、Claude 維持 0。效果：host 記憶體見底時，先砍可以重來的東西。
   ⚠️ 兩個限制要照實寫：① BuildKit 的 build 步驟設不到 adj，所以 build 仍要靠 B1／C1；② Virtuozzo 的 OOM 是否照 `oom_score_adj` 挑對象
   ⛔ 沒有實測，而且⛔ 不能安全地實測（逼出 OOM 會波及 live），只能驗證值有沒有套上去。
   **實作細節**（v5 補充；✅ 使用者 2026-11-02 確認）：
   * 共用參數放在 `scripts/lib/mem-guard.sh`（三支 `test.sh` 本來就 source 它）：一個函式把 `--oom-score-adj=<值>` 加進呼叫端的 docker 參數陣列。
     另外 8 支 source 它的腳本（migration、replay 系列）⛔ 不呼叫這個函式，行為不變。
   * `TEST_OOM_SCORE_ADJ`（預設 1000）：設成空字串就⛔ 不帶參數——萬一這台 Virtuozzo 的 docker 拒絕它，三支測試會全部起不來，這個開關讓人⛔ 不必改程式就能先繞過。
     值只收 0～1000 的整數，其他值在任何 docker 指令之前就中止；負值⛔ 不收（那是反過來保護測試 container，違背 D4）。
   * dev compose 加在共用的資源限制 anchor，5 個服務（含 profile 的 python-worker）一次套上。
   * 範圍只有三支 `test.sh`：其他會起測試 container 的腳本（migration 測試、replay 系列、`test-redis-emission.sh`、python `test.sh` 內部呼叫的 `test-replay-args.sh`）⛔ 不在這次範圍。
   * 實跑併進交接腳本 `run-smoke.sh`：S1 之後查四個 dev container 的 `HostConfig.OomScoreAdj` 與實際行程的值（從 host 讀 `/proc/<State.Pid>/oom_score_adj`，
     與 container 內的 PID 1 是同一個行程，⛔ 不依賴 image 裡有沒有 `cat`）；收尾 down 之後新增 S5，背景跑一次很小的 `backend/scripts/test.sh`，
     抓到新出現的測試 container 就查同樣兩個值。
2. **dev stack 只在驗收時開**，驗完 `down`（既有做法，寫成規則）。
3. **`mem_limit` 這次⛔ 不調整數值**（v2 修正理由）：
   * 它做得到的：`mem_limit` 是單一 container 的**硬上限**，用量撞到它時由那個 container 的 cgroup 處理（在 container 裡面挑行程砍），
     所以確實能限制單一 container 的用量、**降低**整機 OOM 的風險（[Docker 官方說明](https://docs.docker.com/engine/containers/resource_constraints/)）。
   * 它做不到的：**⛔ 不保證整機的總量夠用**——各 container 的上限加總遠大於 2 GiB，大家都還沒撞到自己的上限時，整機照樣可能見底；
     而且它⛔ 不涵蓋 BuildKit 的 build 步驟，也⛔ 不涵蓋 host 上的程序（Claude、dockerd、codex）。
   * 這次⛔ 不調的理由：① live 的上限屬於 live 的設定，本計畫⛔ 不動 live；② 調整要先量每個服務的峰值（設太低會讓正常的峰值被砍），本計畫沒有這份量測；
     ③ 這次最大的兩個風險來源（build 與 Claude）本來就⛔ 不受它控制，改由 B1、C1 處理。之後要調，另立計畫並先量峰值。
4. **其他常駐程序**（fin-api、codex）是使用者的其他工作，`RACE=1` 那段時間要不要暫停由使用者決定（D1 (b)）。
5. **長期容量**（D6）：只列選項——升級主機的 RAM；把 dev／測試搬到別台；或用 GitHub Actions 跑 race（runner 記憶體充足，但要 push 到 GitHub、
   新增 workflow，屬於對外的動作）。

## 三、決定（✅ 2026-11-02 定案：全部採建議）

| # | 問題 | 選項 | 建議 |
|---|---|---|---|
| D1 | `RACE=1` 由誰、怎麼跑 | (a) 使用者在 Claude 結束後執行；(b) Claude 執行，先停 dev、fin-api、codex；(c) GitHub Actions | (a)：⛔ 不動其他專案，省最多（Claude 本身） |
| D2 | 時段 | 平日 17:05～22:00／週末 | 平日 17:05～22:00（避開 live 批次，當天就能做） |
| D3 | 要不要先確認 detector 有效 | 要（多一次 race 執行）／不要 | 要：文件規定的步驟；⛔ 沒有它，綠燈可能只代表沒壓到 |
| D4 | `oom_score_adj` 的值與範圍 | 測試 1000、dev 500／只加測試／不加 | 測試 1000、dev 500 |
| D5 | Dockerfile 的前端 stage 改用 repo 裡的 dist（省掉 build 時的 `npm ci`） | 做／不做 | 不做：會改到 live 的 image 建置，屬於另一件高影響異動；先看 B1 的實測 |
| D6 | 長期容量 | 升級主機／搬走 dev 與測試／CI 跑 race／維持現狀 | 只記錄，由使用者決定 |

## 四、受影響的檔案與資料流

* `backend/scripts/test.sh`：race 的前置檢查（A4）、`--oom-score-adj`（「二 D」1）。
* `frontend/scripts/test.sh`、`python/scripts/test.sh`：`--oom-score-adj`（「二 D」1）。若三支共用參數，放進 `scripts/lib/`。
* `scripts/smoke-dev.sh`：依序 build、`NO_BUILD`、`SERVICES`、`OVERRIDE`（B1～B4）；新的選項預設都是關閉，現有用法的行為不變（除了 build 改成依序）。
* `docker-compose.dev.yml`：每個服務的 `oom_score_adj`（「二 D」1）。
* `docs/development-workflow.md`：A、B5、C、D 的規則。
* 資料流與 contract：⛔ 沒有變化（只動測試與 dev 的工具；live 的 runtime ⛔ 不受影響）。

## 五、風險與回滾

| 風險 | 對策／回滾 |
|---|---|
| 依序 build 比較慢 | 預計多 1～2 分鐘；可接受。回滾：改回 `compose build` |
| `oom_score_adj` 在 Virtuozzo 沒有效果 | 沒有害處；值有套上就留著。回滾：拿掉參數 |
| race 的前置檢查把原本勉強跑得完的情況擋下來 | `RACE_MIN_MB` 可覆寫；門檻用實測的 700m |
| 使用者執行 race 時複製的指令被換行切斷 | 指令寫成暫存腳本、使用者只執行一行 |
| debian image 佔 800 MB 磁碟 | 剩 8.2 GB；之後還會用到，留著 |
| smoke 的新選項和文件裡舊的程序不一致 | 新選項預設關閉；文件同步改 |

## 六、測試與驗證

* 腳本：`bash -n`；`smoke-dev.sh` 實跑一次（依序 build），build 期間每 2 秒記 MemAvailable，記下低點。
* `smoke-dev.sh` 的各模式（v3）：在 PATH 前面放假的 `docker`、`curl`，記下每次呼叫的參數，驗呼叫順序——預設（除了 build 拆成兩次，與現在相同）、
  `NO_BUILD`、只選 backend、只選 python-server、兩個都選（含逗號分隔）、⛔ 不合法的值（中止、⛔ 沒有任何 docker 呼叫）、`OVERRIDE` 的相對路徑與不存在的檔、
  `--wait` 失敗（印診斷、⛔ 不接著 `--no-deps`）、`SKIP_DOWN=1`。實跑（上一條與下一條）由使用者在 Claude 結束後執行（C1）。
* 部分服務模式（「二 B」3）：① dev 已經 down 時 `NO_BUILD=1 SERVICES=backend` → postgres、redis 由 `--wait` 起到 healthy、backend 健康檢查通過，
  ⛔ 沒有起 python-server、⛔ 沒有檢查 `PYTHON_URL`；② 依賴已經在跑時再跑一次 → `--wait` 立刻返回、只重建 backend；
  ③ `SERVICES=python-worker` 之類的值 → 直接中止；④ 預設（⛔ 不帶 `SERVICES`）→ 與現在相同，兩個健康檢查都跑。
* `oom_score_adj`：`docker inspect` 的 `HostConfig.OomScoreAdj` 與 container 內 PID 1 的 `/proc/1/oom_score_adj`（測試 container 與 dev 各一個）。
* race 的前置檢查：用 `MEM_RESERVE_MB` 把上限壓到 700m 以下，確認直接中止、訊息正確；壓不到時照常進行。
* `RACE=1`：D3 的有效性確認報出 `WARNING: DATA RACE`；正式版 rc 0、⛔ 沒有 `DATA RACE`；log 保存在 repo 外，結果寫進 I-123～I-126 計畫書「十二」。
* `scripts/check-doc-refs.py` 問題 0。

## 七、執行順序

1. 使用者確認本計畫（D1～D6）。
2. I-123～I-126 階段 B 的 review 修正先做（另一份計畫：[`i123-i126-job-retry.md`](./i123-i126-job-retry.md)「十二」）——race 要跑在修正後的程式上。
3. A3：拉 debian image（先問使用者）；A4：race 的前置檢查。
4. A1／A6：Claude 寫好暫存腳本、交給使用者在 D2 的時段執行（先 D3 的有效性確認，再正式版）；新 session 讀 log、寫回結果。（✅ 2026-11-02：第二次執行通過，見「十」）
5. B1～B4、「二 D」1：腳本與 dev compose 的修改 → 驗證（「六」）。（✅ 2026-11-02：實跑通過，見「十」）
6. 文件（B5、C、D）→ review → 使用者 commit。

## 八、完成後的歸檔位置

* `docs/development-workflow.md`：「Race detector」（執行方式、前置檢查、時段）、「`MEM` 是上限，不是預留」（犧牲順序、`mem_limit` 做得到與做不到的事）、
  「2. 啟動 dev stack 做 smoke test」（新選項、cron 的推算時機、被砍之後的接手程序）。
* `backend/scripts/test.sh` 檔頭：race 的前置檢查。
* `scripts/lib/mem-guard.sh` 檔頭：`TEST_OOM_SCORE_ADJ`（已隨「二 D」1 寫入）。
* ⚠️ 程式註解裡指向本計畫書的引用（`backend/scripts/test.sh` 的 A4；三支 `test.sh`、`mem-guard.sh`、`docker-compose.dev.yml` 的「二 D」1），
  收斂、刪除計畫書時要改指 `development-workflow.md` 的對應節，⛔ 不能留下斷鏈。

## 十、實作紀錄

### 2026-11-02

* **A4 已實作**：`backend/scripts/test.sh` 在 `RACE=1` 時，mem-guard 下修後的 `MEM` 低於 `RACE_MIN_MB`（預設 700）就直接中止，印出還差多少。
  用假的 `docker` 指令驗了三條路徑：預設門檻 → 中止（當時 Claude 在跑，MEM 被下修到 492m、還差 208 MB）；`RACE_MIN_MB=100` → 照常走到 `docker run`；
  不開 race → 不受影響。
* **A1／A6 的腳本已備好**（repo 外）：`~/.cache/stock_trading/race-logs/20261102/run-race.sh`。前置檢查：⛔ 沒有 claude 程序、dev stack ⛔ 沒在跑、
  本機有 `golang:1.25`，任一不成立就中止。三步依序跑：① detector 有效性確認（在複本裡做出一個 race）；② `./internal/scheduler/...`；
  ③ `./internal/api/handler/`。每步的 rc 與 `DATA RACE`／`FAIL`／`signal: killed` 的筆數寫進同目錄的 `summary.txt`。
* **D3 的做法與一個發現**：重跑相關的既有測試幾乎都用 channel、atomic 或 WaitGroup 把 goroutine 串起來，**單純拿掉一把鎖多半⛔ 不會被 race detector 抓到**
  （存取之間已經有 happens-before）。初版的改壞方式是：在 `tryTriggerDelisting` 把「記下用掉的代號」（`usedGen`）移到解鎖、`spawn` 之後才寫，
  期望測試 (v)（`TestDelistingLinkUsesEachStockSuccessOnce`）spawn 出來的那一輪對帳在鎖內讀它時報 race。
* **第一次執行（2026-11-02 14:12～14:16，⚠️ 不在 D2 時段）**：記憶體沒問題（MemAvailable 約 930 MB、killed=0、中止=0）。
  ① 有效性確認 DATA RACE=0、rc=0、測試 PASS；② ③ rc=0、全 0。**① 不成立，② ③ 照 D3 ⛔ 不能採計。** log 保存在同目錄的 `run1-1412/`。
  * 讀碼確認 `-race` 有生效（log 印的是 `go test -race`、debian image、`CGO_ENABLED=1`、掛載的是複本），複本的修改也有套上。
  * 原因（讀碼推論）：race detector 只看這次實際執行裡兩次存取之間有沒有 happens-before。父 goroutine 在鎖外寫完之後，還會經過
    `releaseStockSymbolSync` 的 atomic store 與 `afterRelease` 的 `r.mu` 上鎖／解鎖；子 goroutine 讀 `usedGen` 的地方全都在 `r.mu` 內，
    所以寫 → 解鎖 → 上鎖 → 讀有先後關係，⛔ 不算 race。`GOMAXPROCS=1` 也讓子 goroutine 幾乎一定等父 goroutine 卡在 `h.wg.Wait()` 才跑；
    測試 PASS 也吻合這個順序（讀的時候已經寫好）。初版「真正會同時存取的是 spawn 出來的那一輪」這個前提⛔ 不成立。
* **改壞方式的修正（2026-11-02）**：保留鎖外的寫入，另外讓 spawn 出來的 closure **一開頭、還沒碰任何鎖**就在鎖外讀 `usedGen`
  （讀到的值拿去做條件判斷，避免被編譯器省掉）。兩次存取之間只有 `go` 這個起點，⛔ 沒有其他同步，不論誰先跑都會報 race。
  規則：**改壞的方式要讓兩次存取之間⛔ 沒有任何同步（鎖、atomic、channel、WaitGroup）**；只把寫入移到鎖外，後面還經過同一把鎖就抓不到。
  驗證：`bash -n` 通過；隔離複本套用後與 repo 只差預期的兩處；一般模式下編得過、測試 (v) PASS。⚠️ 那次 rc=1，原因是 Claude 在跑、
  mem-guard 把上限下修到 550m，`go test` 內建 vet 跑 `modernc.org/sqlite/lib` 時被砍；之前的 `go vet` 有通過（腳本 `set -e`，測試能跑代表它結束碼是 0）。
  `RACE=1` 的效果要等使用者在 D2 時段重跑才知道；① 若仍是 0，就停下來重查，⛔ 不採計 ② ③。
* **涵蓋範圍的修正**：連帶觸發這條路徑在既有測試裡同樣有 happens-before，初版的改壞方式在這些測試下就抓不到。所以正式版的 race 綠燈只代表
  「這些測試實際產生的交錯裡沒有 race」，⛔ 不代表連帶觸發的路徑已經驗過，也⛔ 不等於所有交錯都驗過。
  「八」歸檔到 `development-workflow.md`「Race detector」時，要補上上面那條改壞方式的規則。
* **A3 已完成**（使用者同意後，2026-11-02）：`golang:1.25`（linux/amd64，image id `sha256:9bae33b0…`，約 848 MB）；磁碟剩 7.2 GB。
* **第二次執行（2026-11-02 14:28～14:29，使用者在 Claude 結束後執行；⚠️ 不在 D2 時段）**：程式是 HEAD `389f652`（改壞方式的修正，14:27:54 commit；
  工作樹乾淨）。**① 對照組成立、② ③ 通過 → 依 D3 採計 ② ③，G1 達成。** 新 session 讀 log 判讀（唯讀）。
  * 三步的結果（`summary.txt`）：① detector（複本、測試 (v)）rc 1、DATA RACE 1、FAIL 1（抓到 race 時預期如此）；
    ② `./internal/scheduler/...` 與 ③ `./internal/api/handler/` 都是 rc 0、DATA RACE 0、FAIL 0；三步的 killed 與中止都是 0。
  * ① 的 race 落在注入的兩處：讀取是 spawn 出來的 closure（`tryTriggerDelisting.func1`）開頭的鎖外讀，先前的寫入是 `tryTriggerDelisting` 在 spawn 之後的鎖外寫，
    讀取端的 goroutine 由 `r.spawn` 建立。FAIL 只來自 `race detected during execution of test`，⛔ 沒有其他斷言失敗。
    複本與 repo 全目錄比對只差 `job_retry.go` 注入的兩處，`scripts/lib` 相同——① 與 ② 測的是同一份程式，只差注入。
  * ② ③ 確實有跑：都是 `ok`，`go test` 報 44.280s、18.494s；`internal/scheduler` ⛔ 沒有子套件，正式版⛔ 沒有帶 `-run`；handler 套件有 18 個測試檔；
    兩份 log 都⛔ 沒有 `[no tests to run]` 或 `[no test files]`。
  * 記憶體：MemAvailable 940 → 935 MB（三步開始時 939／936／936 MB）；mem-guard 三步都給 `MEM=700m`（上限 787／785／784 MB），A4 的前置檢查⛔ 沒有擋下。
  * 時段：⚠️ 不在 D2（17:05～22:00）。執行時間在 13:30 收盤之後、15:00 `daily_close` 之前（`intraday` 的 cron 到 13:55 仍會觸發，
    但 `IsMarketOpen` 在 13:30 之後就跳過）；⛔ 沒有查 `/scheduler/status`，⛔ 未確認當時是否仍有批次在跑。
  * 印證了上面「改壞方式的修正」那條規則：只把寫入移到鎖外（第一次）報 0；兩次存取之間⛔ 沒有任何同步（第二次）報 1。
  * 涵蓋範圍照上面「涵蓋範圍的修正」：② ③ 的綠燈只代表這些測試實際產生的交錯裡沒有 race，⛔ 不等於所有交錯都驗過。
  * 結果已寫回 [`i123-i126-job-retry.md`](./i123-i126-job-retry.md)「十二」。log 在 repo 外的 `~/.cache/stock_trading/race-logs/20261102/`
    （`summary.txt`、三步各一份 log、① 用的複本 `detector-copy/`、第一次執行的 `run1-1412/`），保存到本計畫與 I-123～I-126 階段 B 的 review 完成。
* **B1～B4 已實作、✅ 程式 review 通過**（2026-11-02，`scripts/smoke-dev.sh`，依「二 B」1～4 與 6；review 含下面交接腳本的修正；✅ 實跑通過（2026-11-02，見下方「實跑」））：
  * 依序 build，只 build 選了的服務；`NO_BUILD` 跳過 down 與 build，`up` 加 `--no-build`；`SERVICES` 只選一個時先用 `--wait` 等依賴 healthy，再用 `--no-deps` 起它，
    健康檢查與診斷只看選了的服務，只選 backend 時印提醒；`OVERRIDE` 的相對路徑以呼叫時的目錄解析，檔案不存在就中止。檔頭的 usage 補上新變數，既有變數的行為不變。
  * 輕量驗證（⛔ 沒有碰真的 docker）：`bash -n` 通過（這台 host ⛔ 沒有 shellcheck）。PATH 前面放假的 `docker`、`curl`，記下每次呼叫的參數，20 個情境全部符合預期——
    預設、`NO_BUILD`、只選 backend、只選 python-server、兩個都選（逗號、重複、空字串、只有分隔符號）、⛔ 不合法的值 3 種（rc 2、⛔ 沒有任何 docker 呼叫）、
    `OVERRIDE` 的相對路徑與絕對路徑、不存在的檔與目錄（rc 2）、`--wait` 失敗（印 postgres、redis 的 log，⛔ 沒有接著 `--no-deps`）、`SKIP_DOWN=1`、
    部分與全部模式的健康檢查失敗、build 失敗（停在第一個 build）。
  * 與 HEAD 的舊版對照：預設與健康檢查失敗兩條路徑，除了 build 拆成 `build backend`、`build python-server` 兩次，其餘呼叫相同。
    對照組：拿掉 `--no-deps` 的複本跑「只選 backend」→ 比對抓到差異，比對本身有效。
  * v4 的 `COMPOSE_PROJECT_NAME` 守門：設成 `stock_trading`、空字串、搭配部分服務模式三種，都是 rc 2、⛔ 沒有任何 docker 呼叫；
    HEAD 的舊版在同樣設定下照常 down、build、up（對照組）。連同上面共 27 項全部符合預期。
* **交接腳本 `run-smoke.sh` 的 review 修正**（2026-11-02，review 發現 2 項，都已用假的 `docker` 重現；腳本在 repo 外）：
  * P1 沒有固定 dev 的檔案與 project：繼承了 `COMPOSE_FILE`、`COMPOSE_PROJECT_NAME`，S1 的 `down --remove-orphans` 會被導到別份 compose 或別的 project。
    修正：開頭清掉所有 `COMPOSE_*` 環境變數；compose 一律帶 `-p stock_trading_dev -f <dev compose>`；呼叫 `smoke-dev.sh` 時把 `COMPOSE_FILE` 明確設成 dev compose；
    前置檢查用 `config --no-interpolate` 確認 dev compose 解析出的 project 是 `stock_trading_dev`（查詢失敗或名稱不符就中止）；`DOCKER_HOST`、`DOCKER_CONTEXT` 有設定就中止。
    這項也查出 `smoke-dev.sh` 本身的缺口，見「二 B」6 的 v4。
  * P2 查詢失敗被當成「沒有 container」：`docker ps`／`inspect` 前後都失敗時，摘要顯示「0 個」「前後相同＝是」、rc 0。
    修正：查詢函式一律檢查結束碼，「查詢失敗」與「確實沒有」分開。執行前失敗就中止；之後失敗照常收尾 down，摘要標「查詢失敗」，最後 rc 非 0。
    同一類的另外三處一起改：S2 與 S4 的 `compose ps`、S3 的啟動時間（原本兩次都查不到時都是 `none`，會顯示「未重啟＝是」）、S3 的 backend 環境變數。
  * 驗證（假的 `docker` 照真實的優先順序回 project 名稱、可指定第幾次查詢失敗）：設了 `COMPOSE_FILE`、`COMPOSE_PROJECT_NAME`、`COMPOSE_PROFILES` 時，
    新版所有 compose 呼叫都帶 dev compose，也都看不到 `COMPOSE_PROJECT_NAME`；`DOCKER_HOST`、`DOCKER_CONTEXT`、project 名稱不符、`config` 失敗、
    執行前 `ps` 或 `inspect` 失敗 → 中止、⛔ 沒有 down；執行後查詢失敗 → 標「查詢失敗」、rc 1；正常路徑 rc 0，環境變數只數那一個鍵、⛔ 沒有寫進任何輸出檔。
    對照組：review 時的版本在同樣設定下重現了兩項（down 帶著 `COMPOSE_PROJECT_NAME`、`smoke-dev.sh` 的 `-f` 指向別份檔；「0 個」「前後相同＝是」、rc 0）。
  * **實跑**（「六」的依序 build 低點與部分服務模式 ①～④；✅ 2026-11-02 執行，結果見下方「實跑」）：腳本在 repo 外的 `~/.cache/stock_trading/smoke-logs/20261102/run-smoke.sh`，
    由使用者在 Claude 結束後執行（C1），建議平日 17:05～22:00。四步：
    S1 預設（涵蓋 ④；每 2 秒記 MemAvailable，對照兩個 build 的起訖找低點，記 `CACHED` 行數——cache 熱的低點⛔ 不能代表冷 build）；
    S2 情境 ①；S3 情境 ② 加 `OVERRIDE`（只設 `LOG_RETENTION_DAYS=13`，驗 postgres、redis ⛔ 沒有重啟、只重建 backend、值有進 container，只數那一個鍵、⛔ 不印環境變數）；
    S4 情境 ③；收尾 down（⛔ 不帶 `-v`），非 dev 的 container 前後對照。腳本先用假的 `docker` 空跑過一次，流程與摘要行正常。新 session 讀 `summary.txt` 寫回這裡。
* **「二 D」1 已實作**（2026-11-02，依「二 D」1 的 v5 實作細節；✅ review 通過、✅ 實跑通過（2026-11-02，見下方「實跑」））：
  * `scripts/lib/mem-guard.sh` 新增 `mem_guard_oom_arg`：回傳 `--oom-score-adj=<TEST_OOM_SCORE_ADJ，預設 1000>`；空字串回傳空字串（⛔ 不帶參數）；
    其他值（非數字、負值、超過 1000）印錯誤並回 1。前導零先去掉再看位數，`0999` 視為 999。
  * 三支 `test.sh` 在 mem-guard 之後、任何 docker 指令之前取得參數（不合法就中止），加進 `docker run` 的參數；檔頭各補一行說明。
  * `docker-compose.dev.yml` 的 `x-dev-resource-limits` 加 `oom_score_adj: 500`（附註解寫明兩個限制）。
  * 輕量驗證（⛔ 沒有起真的 container）：
    * 四支腳本 `bash -n` 通過。
    * 假的 `docker`（逐字記錄參數），在 scratchpad 的新舊兩棵目錄樹跑（doc-refs 等非 docker 步驟以 stub 代替，`MEM_FORCE=1` 排除 mem-guard 的差異）：
      backend 一般與 `RACE=1`、frontend（`VITEST_ARGS`，npm ci＋vitest 兩次 run）、python（`SKIP_SHELL_TESTS=1`）——
      新版的每個 `docker run` 都帶 `--oom-score-adj=1000`，拿掉它之後與 HEAD 的舊版逐字相同；`TEST_OOM_SCORE_ADJ=` 時與舊版逐字相同；
      python 的 `docker build` ⛔ 沒有帶（BuildKit 的限制，照預期）。
    * 不合法的值（`abc`、`1001`、`-5`、`10000`、`' 5'`、`5x`、`00000001001`）：三支都 rc 1、⛔ 沒有任何 docker 呼叫；
      `0`、`00100`、`1000`、`0999`、`0000` 收下，分別帶 0、100、1000、999、0。
    * `docker compose -f docker-compose.dev.yml --profile worker config --no-interpolate`：5 個服務都是 `oom_score_adj: 500`。
  * 實跑併進 `run-smoke.sh`（✅ 使用者選擇）：S1 之後查 postgres、redis、backend、python-server 的 `HostConfig.OomScoreAdj` 與 `/proc/<State.Pid>/oom_score_adj`（預期 500/500）；
    收尾 down 之後的 S5 背景跑 `backend/scripts/test.sh ./pkg/timeutil/`，每 0.5 秒找新出現的 `golang:1.25-alpine` container（排除執行前就在的），查同樣兩個值（預期 1000/1000）。
    查詢失敗照 P2 的規則標「查詢失敗」、最後 rc 非 0；docker 拒絕參數時會顯示「沒抓到測試 container」與測試的 rc。
    假的 `docker` 空跑 7 條路徑都符合預期：正常（S5 讀的是真的行程，值由參數設定）、docker 拒絕參數、`inspect` 失敗、`ps` 失敗、
    使用者 shell 設了 `TEST_OOM_SCORE_ADJ`（⛔ 不影響 S5）、沒有 dev container、dev 的值不符。⚠️ 這支腳本已改，要再 review。
  * ⛔ 不在範圍：三支 `test.sh` 以外會起測試 container 的腳本（migration 測試、replay 系列、`test-redis-emission.sh`、python `test.sh` 內部呼叫的 `test-replay-args.sh`）。
  * **review 修正（P2）：S5 沒有確認讀到的是這次的 container**（2026-11-02；repo 的參數接入、數值邊界、三支腳本的預設／停用／非法值、dev 的 500 都 ✅ review 通過）：
    原本取「同一個 image 新出現的第一個 ID」，別的同 image container 同時出現時會讀到它的值——這次的測試被拒絕、對方剛好是 1000 時，還會顯示 1000/1000。
    修正：用 `--cidfile` 精準配對、⛔ 不再用 image 篩選。`backend/scripts/test.sh` 的參數是寫死的，所以只在這次測試的子行程裡於 PATH 最前面放一個轉接用的 `docker`，
    只對 `docker run` 加 `--cidfile`、其他參數原樣交給真的 docker（`--oom-score-adj` 仍來自 `test.sh`；⛔ 不改 repo）。沒有 cidfile → 「這次的測試 container 沒有建立」；
    有 cidfile 但查不到 → 「查詢失敗」（docker 在啟動時拒絕參數也是這樣）；兩者都讓最後的 rc 非 0。
    驗證（假的 `docker` 會寫 cidfile、模擬建立時與啟動時的拒絕，以及同 image 的別的 container）：對照組——修正前的版本在「這次被拒絕、別的是 1000」時顯示 1000/1000，
    在「同時有別的是 777」時讀到 777；新版前者顯示「沒有建立」、rc 1，後者讀到這次的 1000/1000（摘要的 container ID 與 cidfile 一致、與別的不同）；
    啟動時被拒絕 → 「查詢失敗」、rc 1；`inspect` 失敗 → rc 1；正常 → rc 0；使用者 shell 設了 `TEST_OOM_SCORE_ADJ` 也⛔ 不影響。轉接的 `docker` 只加了 `--cidfile`，其餘參數與 `test.sh` 給的相同。
  * **review 修正（P2）：cidfile 一出現就只查一次，會把還在啟動的 container 判成查詢失敗**（2026-11-02）：docker 先建立 container、寫 cidfile，之後才啟動，
    這段空窗的 `State.Pid` 是 0，原本的檢查把 Pid 0 當成查詢失敗。
    修正：檢查函式分三種結果——讀到值；查詢失敗；container 存在但還沒有行程（印 `State.Status`）。S5 遇到「還沒有行程」就每 0.2 秒再查，直到啟動、查不到、
    測試結束或 600 秒，後三者分別標「查詢失敗」「沒有啟動」「等不到」，都讓最後的 rc 非 0。同一類的 dev container 檢查：Pid 0 改標「沒有在跑（狀態）」，
    ⛔ 不等待（健康檢查通過後本來就該在跑），判讀算無法判定。
    驗證（假的 `docker` 在寫 cidfile 之後先停在 `created` 一段時間）：對照組——修正前的版本在有空窗時判成「查詢失敗」、rc 1，dev 的 Pid 0 也標成「查詢失敗」；
    新版有空窗時等到啟動後讀到 1000/1000、rc 0（等待期間查了 10 次）；空窗後啟動被拒絕 → 「查詢失敗」、rc 1；一直停在 `created` 而測試結束 → 「沒有啟動」、rc 1；
    逾時（把等待上限縮短的複本）→ 「等不到」、rc 1；dev 的 Pid 0 → 「沒有在跑（restarting）」、rc 1；沒有空窗與建立時被拒絕的路徑結果不變。
  * **review 修正：逾時後仍會無限等待測試子程序**（2026-11-02；✅ review 通過；「沒有啟動」也清 container 經使用者確認保留）：輪詢結束後無條件 `wait`，docker 卡在啟動時永遠不會結束，走不到「等不到」的判定。
    修正：S5 的測試改由 `timeout -k 10s 900s` 啟動（它自成 process group，逾時或收到 TERM 時轉給整個測試，10 秒後補 KILL），任何路徑的 `wait` 最多 910 秒；
    等 cidfile 與等啟動共用 600 秒的期限；逾時時先記錄「等不到」（分「沒有 cidfile」與「停在 created」），再對 `timeout` 送 TERM、最多等 20 秒（仍在就 KILL 它並註記「可能殘留」）、回收。
    另外清掉這次的 container（✅ 使用者確認）：`timeout 30s docker rm -f <cid>`，只對 cidfile 讀到、驗過格式的 64 碼 ID；「沒有啟動」（測試結束時 container 仍停在 `created`）
    也一併清，因為同樣是 `--rm` 處理不到的狀態。
    驗證（假的 `docker` 可卡在建立之前或啟動、可忽略 TERM；期限縮短的隔離複本）：對照組——修正前的版本卡在 `wait`，被外層在 25 秒時 KILL（rc 137）；
    新版卡在啟動 → 3 秒走到「等不到」、測試 rc 143、清掉這次的 container；卡在建立之前 → 「等不到建立」、⛔ 沒有呼叫 `rm`；忽略 TERM → 13 秒（10 秒後補 KILL）、測試 rc 137；
    「沒有啟動」→ 清掉 container；正常路徑結果不變；每條路徑之後都⛔ 沒有殘留的假行程。「`timeout` 本身 20 秒內⛔ 沒有結束」的後備路徑⛔ 沒有實測（`timeout` 送完 KILL 就會結束，模擬不出來）。
    同類掃過：S1～S4 在前景呼叫 `smoke-dev.sh`，它的健康檢查有 `WAIT_SECONDS` 上限，build 與 up 則沒有（build 本來就可能很久）——屬於 `smoke-dev.sh` 本身的行為，這次⛔ 不動。
* **實跑（2026-11-02 16:20～16:25，使用者在 Claude 結束後執行；⚠️ 不在 D2 時段）**：B1～B4 與「二 D」1 ✅ 通過。新 session 讀 log 判讀（唯讀）。
  * 前置：MemAvailable 899 MB；非 dev 的 container 7 個。summary ⛔ 沒有「有查詢失敗」那行（腳本 rc 0）。
  * S1 預設：rc 0、passed 1，16:20:27～16:24:28。依序 build：backend 16:20:27 開始、python-server 16:24:10、up 16:24:11。
    四個 dev container（postgres、redis、backend、python-server）的 `HostConfig.OomScoreAdj` 與實際行程的值都是 500/500。
  * S2 情境 ①（dev 已經 down → `NO_BUILD=1 SERVICES=backend`）：down rc 0、rc 0、passed 1、提醒 1 次、⛔ 沒有檢查 python、⛔ 沒有 build；
    `compose ps` 只有 backend running、postgres 與 redis running healthy，⛔ 沒有 python-server。
  * S3 情境 ② 加 `OVERRIDE`：rc 0、passed 1、⛔ 沒有 build、等依賴 0 秒；postgres、redis ⛔ 沒有重啟，只重建 backend；backend 有 `LOG_RETENTION_DAYS=13`（1 筆）。
  * S4 情境 ③（`SERVICES=python-worker`）：rc 2、錯誤訊息 1 行、`compose ps` 前後相同。收尾 down（⛔ 不帶 `-v`）rc 0。
  * S5 測試 container：測試 rc 0；cidfile 對到的 container `746197e122ec`，`HostConfig.OomScoreAdj` 與實際行程的值都是 1000/1000。
    `./pkg/timeutil` 回報 `[no test files]`，但 `go vet` 與 `go build` 有跑、container 有起來——S5 只用來起測試 container，⛔ 不影響判讀。
  * 非 dev 的 container 7 個：名稱、ID、啟動時間、重啟次數前後相同——未觀察到 container 重建或重啟；⛔ 未驗證 OOM 事件及批次影響。結束時 MemAvailable 898 MB。
  * **「二 D」1 的結論**：這台 Virtuozzo 的 docker 接受 `--oom-score-adj`，dev 與測試 container 都起得來、值也套上了。
    ⚠️ OOM 真的發生時 kernel 是否照這個值挑對象，仍⛔ 沒有實測（「二 D」1 的限制 ②）。
  * **B1 只驗了一半**：
    * 確認了：兩個 image 依序 build。整段的 MemAvailable 低點 242 MB @16:23:02，落在 backend 的 `go build ./cmd/server`（181 秒；
      期間 91 次取樣有 25 次低於 400 MB）；`npm run build` 那 16 秒的低點是 592 MB。
    * 沒量到：`npm ci`、`go mod download` 與 python 的 `pip install` 都命中 cache（`CACHED` 13 行：backend 9、python-server 4），python-server 1 秒就 build 完（低點 858 MB），
      所以「平行 build 時兩個峰值會不會疊加」這次量不出來。之後 python 的依賴改變（`pip install` ⛔ 不命中 cache）時，照同一支腳本的 S1 再量一次；
      歸檔到 `development-workflow.md` 時列為已知限制，⛔ 不另立 todo。
    * 這次 Claude ⛔ 沒在跑，242 MB ⛔ 不能直接和舊紀錄的冷 build 低點（74 MiB）比。反過來說，Claude 在跑時（280～400 MB），這一步會把 MemAvailable 壓到接近 0——
      印證 C1「重的步驟⛔ 不跟長 session 疊在一起」。
  * 時段：⚠️ 不在 D2（17:05～22:00）。16:00 的 `evaluation_universe_sync` 若 live 有開（dev compose 的註解：約 27 分鐘），可能和這次重疊——⛔ 沒有查 live 的開關；
    前後對照只確認未觀察到 container 重建或重啟，⛔ 未驗證 OOM 事件及批次影響。
  * log 在 repo 外的 `~/.cache/stock_trading/smoke-logs/20261102/`（`summary.txt`、各步的 log、`S1-mem.log`、`s5.cid`、`s5-shim/`），保存到本計畫 review 完成。

## 九、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-02 | 初版 |
| v2 | 2026-11-02 | 依 review 修正：#1「二 B」3 補部分服務模式的規則——依賴的啟動順序（`--wait` 等到 healthy 才 `--no-deps`）、dev 已經 down 時也成立、健康檢查只看選了的服務、⛔ 不需要 python 的限制；「二 B」2 改成 `NO_BUILD` 只跳過 down 與 build；「六」補四個情境。#2「二 D」3 改寫 `mem_limit` 的理由：硬上限能降低風險，但⛔ 不保證整機總量、⛔ 不涵蓋 BuildKit 與 host 上的程序；維持本次⛔ 不調整數值；「八」同步。另把 A6 的描述對齊「十」的實際做法 |
| v3 | 2026-11-02 | 實作 B1～B4 前補「二 B」6 的實作細節（✅ 使用者確認）：部分服務模式只 build 選了的服務；`NO_BUILD` 時 `up` 加 `--no-build`；`OVERRIDE` 的路徑解析、存在檢查與單一檔；`SERVICES` 的分隔與空值；診斷印哪些 log；模式摘要。「六」補假 `docker` 的模式驗證 |
| v4 | 2026-11-02 | 依 review（交接腳本 `run-smoke.sh` 的 P1：沒有固定 dev 的檔案與 project）查出 `smoke-dev.sh` 本身也會被 `COMPOSE_PROJECT_NAME` 導到別的 project（改版前就有）；「二 B」6 補「有設定就中止」（✅ 使用者確認） |
| v5 | 2026-11-02 | 實作「二 D」1 前補實作細節（✅ 使用者確認）：共用參數放在 `mem-guard.sh`；`TEST_OOM_SCORE_ADJ` 的預設、關閉與合法範圍；dev compose 加在 anchor；範圍只有三支 `test.sh`；實跑併進 `run-smoke.sh`（S1 之後與新的 S5） |
