# 2 GiB host 的 OOM 改善 計畫 v2（2026-11-02，✅ 決定 D1～D6 依建議定案（2026-11-02）；v2 依 review 修正「二 B」2、3 與「二 D」3）

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
4. A1／A6：Claude 寫好暫存腳本、交給使用者在 D2 的時段執行（先 D3 的有效性確認，再正式版）；新 session 讀 log、寫回結果。
5. B1～B4、「二 D」1：腳本與 dev compose 的修改 → 驗證（「六」）。
6. 文件（B5、C、D）→ review → 使用者 commit。

## 八、完成後的歸檔位置

* `docs/development-workflow.md`：「Race detector」（執行方式、前置檢查、時段）、「`MEM` 是上限，不是預留」（犧牲順序、`mem_limit` 做得到與做不到的事）、
  「2. 啟動 dev stack 做 smoke test」（新選項、cron 的推算時機、被砍之後的接手程序）。
* `backend/scripts/test.sh` 檔頭：race 的前置檢查。

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

## 九、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-02 | 初版 |
| v2 | 2026-11-02 | 依 review 修正：#1「二 B」3 補部分服務模式的規則——依賴的啟動順序（`--wait` 等到 healthy 才 `--no-deps`）、dev 已經 down 時也成立、健康檢查只看選了的服務、⛔ 不需要 python 的限制；「二 B」2 改成 `NO_BUILD` 只跳過 down 與 build；「六」補四個情境。#2「二 D」3 改寫 `mem_limit` 的理由：硬上限能降低風險，但⛔ 不保證整機總量、⛔ 不涵蓋 BuildKit 與 host 上的程序；維持本次⛔ 不調整數值；「八」同步。另把 A6 的描述對齊「十」的實際做法 |
