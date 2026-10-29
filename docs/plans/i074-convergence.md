# I-074 收斂計畫 v3（2026-10-29；⚠️ 待確認；v1 第一輪 review 的 3 項、v2 第二輪 review 的 1 項已修正，見「十」）

需求確認（2026-10-29）：依 ⑪ 計畫 D3，另立本計畫，把 I-074 從 `docs/issue.md` 整筆收斂。使用者已裁決：

* ⑪ 的判讀結果（判讀 B）**review 通過**，可以排入「移除 I-074」。
* **分兩階段**：階段 1 歸檔到主題文件（review → commit）；階段 2 移除 I-074、修引用、刪計畫書（review → commit）。
* 程式與腳本裡指向 I-074 的註解**一起改、只改註解**。⚠️ 盤點時發現有一部分檔案被測試凍結、改了註解測試就會失敗，見「〇」5 與 D1。
* repo 外的產物**納入**本計畫；刪除放在最後一步，每一項刪除之前都先經使用者確認。

本計畫確認之前，⛔ 不改主題文件、⛔ 不改程式、⛔ 不刪任何檔案（唯一的例外是 I-074 狀態列加一行指向本檔）。

## 〇、唯讀盤點（2026-10-29，HEAD `20eacb2`）

| # | 項目 | 事實 |
|---|---|---|
| 1 | I-074 的篇幅 | `docs/issue.md` 的 I-074 從標題到 I-100 之前的分隔線，共 **11,803 行**、76 個 `####` 節。前言與欄位表約 32 行；其餘節依類型分：① 2026-09-01 的實測與處置（約 380 行）；② Stage 0、Stage 1 的計畫收斂與實作結果（約 460 行）；③ Stage 2 ⑦～⑨-2 的細部計畫、實作結果、執行結果（約 6,800 行）；④ Stage 2 計畫書 v18～v29、②、③ evidence contract v1～v12（約 3,000 行）；⑤ ③b～⑤ 的結果（約 790 行）；⑥ 前置盤點、計次裁決、關閉條件、I-100 前置、測試／風險／歸檔（約 330 行） |
| 2 | 錨定版本 | `20eacb204ac558453360aed342899d4bb2ac30dd`（⑪ 判讀 B 的 commit）同時含完整的 I-074 與 ⑩、⑪、⑨-2 的計畫書；本計畫的「歷史引用」一律指向它（D2） |
| 3 | 文件裡的引用 | `development-workflow.md` 47 處（其中 20 處指向 issue.md）、`sr-zone-scoring.md` 33 處（12 處）、`todo.md` 10 處（3 處）、`plans/evening-batch-shift.md` 2 處；I-074 自己的計畫書：`i074-stage2-step10.md`、`i074-stage2-step11.md`、`i074-stage2-step92-p1.md`、`i074-stage2-step92-s92.md`。issue.md 的其他條目：I-100 26 處、I-116 7 處、I-120 5 處、I-115／I-119／I-121 各 4 處、I-117／I-118 各 3 處、I-078 1 處、檔頭 2 處 |
| 4 | 程式裡的引用 | 約 118 個檔案提到 I-074，多數是名稱（「I-074 Stage 2」、`i074_*` 檔名）。**指向 issue.md 章節**的行：凍結集合以外有 **83 行／53 個檔**（`scripts/` 的 shell 與 Python、`python/scripts/`、`sr_scoring/tests/`、`frontend/src/lib/api/srZones.ts`）；凍結集合裡約 28 行（`evaluation.py`、`replay_bundle/` 24 行，`decision_engine.py`、`lifecycle_engine.py` 各 2 行） |
| 5 | **凍結集合**（盤點時發現） | `scripts/test-replay-args.sh` 的**漂移測試**（由 `python/scripts/test.sh` 呼叫）會以目前 index 的 tree 重新產生 tooling patch，要求與 `python/baselines/i074_stage2/tooling_e1cbbbd.patch` 逐位元相同。產生器 `scripts/make-i074-tooling-patch.sh` 的不變條件 ④ 另外要求：`python/` 扣掉 `python/baselines`、`python/scripts`、`sr_scoring/tests`、`replay_bundle/`、`evaluation.py` 之後，與 `e1cbbbd` **完全相同**。所以 **`python/` 底下除了這三個目錄以外的檔案，改一個位元組測試就失敗**——`decision_engine.py`、`lifecycle_engine.py` 也包括在內；`evaluation.py` 與 `replay_bundle/` 則要重新產生 tooling patch。實際上自 `e1cbbbd`（2026-09-17）以來 Python 產品碼一行都沒有變過；I-074 結束之後，這個凍結仍然有效（D3） |
| 6 | 封存值 | `tooling_e1cbbbd.patch` 的 SHA `5ff9a90f…` ＝ evidence 裡的副本；只有 evidence manifest 記這個值，程式裡⛔ 沒有寫死 |
| 7 | ⛔ 不動的檔案 | `python/baselines/**`（兩份 patch、Stage 1／Stage 2 的 evidence、cohort）、`python/scripts/fixtures/**`；`deploy.sh`、`docker-compose.yml`、`docker-compose.dev.yml`、`backend/config.yaml`、`backend/internal/api/handler/scheduler.go`——後五者只記載「2026-09-01 移除了對 I-074 的引用」，⛔ 不是指向內容，而且是 live 的部署檔 |
| 8 | 文件引用檢查 | `scripts/doc-refs-legacy.txt` 有 47 筆 issue.md 的 line-only 份額；I-074 移除之後多出的份額會讓 `scripts/check-doc-refs.py` 失敗，要同步遞減 |
| 9 | issue.md 檔頭 | 已規定移除程序（「移除條目前要先反轉依賴」與 `rg` 殘留檢查：只准留下明確標成歷史沿革的引用）；「現存條目裡最大的是 I-119」已過時（現在是 I-121） |
| 10 | 主題文件已有的 I-074 節 | `sr-zone-scoring.md`：已知並接受的行為改變（含 ⑪ 的定向驗證）、Decision Replay 的診斷欄位（Stage 0）、Stage 1 的四個永久契約、Stage 2 的環境見證契約、正式證據契約、反事實 replay 路徑與 tooling patch、兩道 guard 與兩份 patch 的封存、晉升與 B／C 判讀器、容量驗收。`development-workflow.md`：Stage 1 的正式執行程序、Stage 2 的環境見證程序、正式證據與 failed record 程序、tooling patch、正式執行程序（含 ⑩ 的貼上版）、sizing harness、acceptance harness 與 observer、測試落點 |
| 11 | 主題文件可能缺的（初判，階段 1 定案） | Stage 1 的正式結果與錨（`MATCH`、D／D+1 artifact 與 manifest 的 SHA；`sr-zone-scoring.md` 0 處）；`M_safety` 的裁定值（`sr-zone-scoring.md` 0 處）；關閉條件的完整界線（before 的兩種形狀、`market_action == AVOID` 的短路、artifact 上⛔ 不可觀測的欄位）；candidate 的精確定義（`sr-zone-scoring.md` 只有 1 處，待核）；部分節的標題仍寫「⚠️ 待 review」，但 I-074 的狀態列記載已經 review 通過，例如「兩道 guard 與兩份 patch 的封存」 |
| 12 | repo 外的產物 | 見「四」；`~/i074_*` 六個目錄合計約 6.0 GB，`/` 的可用空間是 6.1 GB |
| 13 | 四份計畫書自己寫的收斂條件（v2） | `i074-stage2-step10.md`「七」：刪除之前，「十四」裡要長期保留的內容（交給 ⑪ 的錨、**計次的更新**、嘗試的結論）必須已經歸檔；`i074-stage2-step11.md`「七」：與 ⑩、⑨-1、⑨-2 的計畫書在 I-074 收斂時一起處理；`i074-stage2-step92-p1.md`「六」3：P1 的現況規格（「文件」的定義）隨 ⑨-2 的程序搬進 `development-workflow.md` 的 acceptance／sizing 一節；`i074-stage2-step92-s92.md`「五」：工具只服務這一次，⛔ 不搬進主題文件。I-074 內的各計畫節（例如 ⑨-1 v8、⑨-2 v4）也可能寫有收斂條件，由「二」1 逐條找出 |
| 14 | 執行中的工作怎麼辨認（v2） | ⑩ 的容器帶 `i074.stage2.run` label；sizing／acceptance 的容器名稱是 `i074sz-<run id>-…`；⑪ 判讀器的驗證容器以 `docker run --rm` 執行，⛔ 沒有名稱也⛔ 沒有 label，只能以指令列含 `--verify-promotion-staging` 辨認；⑩ 的 sentinel 是 `/run/lock/i074-stage2.active`；sizing／acceptance 的狀態目錄在 `/dev/shm/i074-sizing-<run id>/`（失敗時會保留） |

## 一、目標、範圍與⛔ 不做

* **目標**：I-074 的長期內容（行為、設計、契約、正式值與錨、已知限制、作業程序）都在主題文件裡、可以獨立讀懂；
  I-074 從 issue.md 移除；所有指向它的引用都改指向主題文件或錨定版本；I-074 的計畫書刪除；repo 外的產物依使用者逐項確認處置。
* **範圍**：`docs/issue.md`、`docs/sr-zone-scoring.md`、`docs/development-workflow.md`、`docs/todo.md`、`docs/plans/` 的四份 I-074 計畫書與
  `evening-batch-shift.md`、`scripts/doc-refs-legacy.txt`；凍結集合以外的程式與腳本的**註解與 docstring**；repo 外的產物（「四」）。
* ⛔ 不做：
  * ⛔ 不改任何程式邏輯、字串、測試斷言；⛔ 不動凍結集合（D1）與「〇」7 的檔案。
  * ⛔ 不重跑任何 Stage。
  * ⛔ 不處理 I-100 與 I-115～I-121 本身（只改它們指向 I-074 的引用）；I-100 的收斂另案。
  * ⛔ 不退役 Stage 2 的工具與漂移測試（D3 只登記）。
  * ⛔ 不刪 docker image；⛔ 不處理 `/tmp` 下的 worktree 登記（屬 I-118）。
  * ⛔ 不精簡主題文件既有的 I-074 節（D4）。
  * Claude ⛔ 不 commit。

## 二、階段 1：歸檔（只補主題文件，⛔ 不動 I-074）

1. **逐節分類**：分類對象有兩部分（v2 補上第二部分），結果寫進本檔「十二」（每一節一列：類別、依據）。
   * I-074 的前言、欄位表與 76 個節。
   * 階段 2 會刪除的四份計畫書（`i074-stage2-step10.md`、`i074-stage2-step11.md`、`i074-stage2-step92-p1.md`、`i074-stage2-step92-s92.md`），逐節分類。

   分成三類：
   * **A（已歸檔）**：現況規格已在主題文件。依據要寫出主題文件的節名，並以關鍵詞或關鍵值確認內容確實在那裡。
   * **N（需補）**：長期內容還不在主題文件。依據要寫出補到哪一節。
   * **H（歷史）**：計畫書、review 修訂、實作經過、執行紀錄、已撤回的定義。⛔ 不搬，以錨定版本 `20eacb2` 保留。
   * 一節同時有 A／N 與 H 的內容時，以 A／N 為準，並寫明哪一部分是 H。
   * ⚠️ 判斷依據是「移除之後，讀者只看主題文件能不能正確使用或修改這套工具與結論」，⛔ 不是「這段寫得好不好」。
   * 已撤回的定義⛔ 不搬（例如以 `market_action` 分類、`market_state` 的翻轉條件）；分類與判讀一律以 artifact 的權威欄位為準。
   * **收斂條件（v2）**：I-074 內各計畫節與四份計畫書自己寫的收斂條件或歸檔要求（「〇」13），逐條列成一張表，寫明滿足的方式。
     例如 ⑩「七」的計次更新補到哪一節，或 s92「五」寫明⛔ 不搬。
     有任何一條⛔ 沒有滿足，就⛔ 不進階段 2。
2. **補 N 類**：寫成主題文件的現況說明，⛔ 不寫「issue.md 曾經…」這類敘事。初判的候選見「〇」11。
3. **校正狀態標記**：主題文件 I-074 節標題裡的「⚠️ 待 review」，依 I-074 狀態列與 git 紀錄改成實際狀態（附 commit）。
4. **關鍵值對照**：列一張表，涵蓋 Stage 1、③c、⑤、⑨、⑨-1、⑨-2、⑩、⑪ 的錨與正式值（SHA、列數、`P_B`、`M_safety`、判讀結果）。
   每一個值都要以 `grep` 確認出現在主題文件裡，結果寫進「十二」。
5. 驗證：`python3 scripts/check-doc-refs.py`；`git diff --check`；新增的行裡「⛔」後面都帶否定詞。
6. 停下等 review。review 通過之後，使用者 commit（含本檔與主題文件）。

## 三、階段 2：移除、修引用、刪計畫書

### 三之一、引用的改寫規則

| 規則 | 對象 | 寫法 |
|---|---|---|
| R1 規格指標 | 指向契約、行為、作業程序的引用 | 改指向主題文件的節，例如「見 `sr-zone-scoring.md`「I-074 Stage 2 的正式證據契約」」 |
| R2 歷史指標 | 指向計畫書、review、實作經過、執行紀錄、測試 id 出處的引用 | 改成「（歷史：`git show 20eacb2:docs/issue.md` 的 I-074「…」）」；計畫書寫 `git show 20eacb2:docs/plans/<檔名>`（D2） |
| R3 名稱 | 「I-074 Stage 2」這類名稱、`i074_*` 檔名、「I-074 的 OOM」這類沿革 | ⛔ 不改（編號只增不重用，⛔ 不會混淆） |
| R4 凍結 | 凍結集合（「〇」5）與「〇」7 的檔案 | ⛔ 不改（D1） |
| R5 字串 | 程式裡的字串與訊息（⛔ 不是註解或 docstring） | ⛔ 不改；執行時列出有哪些 |

### 三之二、步驟

1. **issue.md**：
   * 刪除 I-074 整筆（標題到 I-100 之前的分隔線）。
   * 檔頭：收斂紀錄補「I-074 於 <日期> 收斂」；「現存條目裡最大的是 …」改成正確值；殘留檢查那段的「預期出現的 ID」補上 I-074（已收斂）；
     依 D1 補一句凍結集合的例外。
   * 其他條目（I-078、I-100、I-115～I-121）指向 I-074 的引用，依 R1／R2 改寫。
2. **主題文件、todo.md、`evening-batch-shift.md`**：依 R1／R2 改寫。主題文件要寫明凍結集合裡的舊引用該怎麼讀（D1），並列出這些舊引用指的是哪些節：
   `sr-zone-scoring.md` 的 I-074 節開頭加一段「程式註解裡的 issue.md I-074「…」」對照。
3. **程式註解**：凍結集合以外的 83 行／53 個檔，依 R1／R2 改寫。只改註解與 docstring。
4. **刪計畫書**：`git rm` 四份 I-074 計畫書；指向它們的引用依 R2 改寫。
5. **D3**：`todo.md` 新增 T-077，`下一個新編號` 改成 T-078。
6. **基準線**：依 `check-doc-refs.py` 的輸出遞減 `scripts/doc-refs-legacy.txt` 的份額（只准變短）。
7. 驗證（「六」）→ 停下等 review。
8. review 通過 → 「四」的 repo 外處置（逐項確認）→ 結果寫進本檔「十三」，主題文件裡提到這些路徑的地方改成實際的處置結果 → 使用者 commit 階段 2。
9. 本檔的刪除：階段 2 commit 之後、review 確認，在下一次收斂的 commit 刪除（可以併入其他工作的 commit）。I-074 移除之後，issue.md ⛔ 不再有指向本檔的行。

## 四、repo 外的產物（階段 2 的最後一步；**每一項都先經使用者確認**）

| 項目 | 大小 | 內容 | 建議 | 理由 |
|---|---:|---|---|---|
| `~/i074_stage1_run/` | 152M | Stage 1 的 D、D+1、crossday、probe 與各步驟的腳本、log | 刪 | 證據已在 `python/baselines/i074_stage1/`、結果已在 git |
| `~/i074_stage2_run/` | 75M | ③c 的環境見證（witness、envcheck、pin） | 刪 | envcheck 已進版控（`5bae980`） |
| `~/i074_stage2_sizing/` | 3.0G | ⑤ 的 formal、各 validation、⑨-2 的 `formal-92-…`、`.ctl` 與 raw-manifest | 刪 | 正式值與紀錄已在文件；⑩ 已用過 freeze record |
| `~/i074_stage2_acceptance/` | 2.5G | ⑦d 的 dev4～dev7、⑨-1 的 `formal-91-…`、`.ctl` 與 raw-manifest | 刪 | 同上 |
| `~/i074_stage2_formal/` | 317M | ⑩ 的 `<work>`、`.ctl`、⑪ 的 `judge-…` | 刪 | evidence 已 commit；判讀器可以重跑（⑪ D2：保存到 I-074 收斂） |
| `~/i074_stage2_observe/` | 16K | ⑩ 的 observer 輸出 | 刪 | 摘要已在 ⑩ 的紀錄 |
| `~/.local/bin/s11`、`s91`、`s92` | 44K | 簡易指令 | 刪 | 核對對象（計畫書與 I-074 的區塊）在階段 2 之後就不在 HEAD |
| `/dev/shm/s11.lock`、`s91.lock`、`s92.lock`、`/run/lock/i074-stage2.lock` | 0 | 鎖檔 | 刪 | 依下方的檢查確認沒有任何工作在執行、`flock -n` 拿得到鎖；拿不到 → 停下回報 |
| `~/.local/share/stock_trading/i074_stage2/`（795M）、`…/i074_stage1/`（8K） | 803M | Stage 2 image 的 tarball 與 sidecar、Stage 1 的 identity | **保留** | 判讀器重跑需要 image `2a90ad1c…`；tag 被重建之後只能從 tarball 還原 |
| docker image `2a90ad1c…`（`stock_trading-python-server:latest`） | 816MB | | **⛔ 不刪** | dev compose 也在用 |

刪除的程序：
* **先確認沒有任何 I-074 的工作在執行**（v2 補齊；v3 統一判定方式）：沒有符合下列辨認條件的容器與程序、sentinel 與狀態目錄都不存在、四個鎖都拿得到，
  三者同時成立才可以刪；任一條件不符，或任一個查詢失敗，就停下回報、⛔ 不刪（查詢失敗⛔ 不能當作「沒有」）。
  * 容器：以 `docker ps -a --no-trunc` 列出全部容器，依下面四種方式辨認；⛔ 不得有任何一個符合。
    * 帶 `i074.stage2.run` label 的容器（⑩）。
    * 名稱以 `i074sz-` 開頭的容器（sizing／acceptance）。
    * 指令列含 `--verify-promotion-staging` 的容器（⑪ 判讀器的驗證容器，⛔ 沒有名稱與 label）。
    * 指令列含 `backtest.modular.sr_scoring.evaluation` 或 `sr_scoring.replay_bundle.` 的容器（replay、finalize、晉升）。比對到 dev 或 live 的容器時，同樣停下回報、⛔ 不自行判斷。
  * sentinel：`/run/lock/i074-stage2.active` ⛔ 不存在。
  * 狀態目錄：`/dev/shm/i074-*` 是空的。若有，代表 sizing／acceptance 有失敗後保留的 S，要依 `development-workflow.md` 的「保留的 S 的刪除程序」另行處理，⛔ 不在本步驟刪。
    ⑩ wrapper 的控制目錄 `/dev/shm/s10-*` 也⛔ 不存在。
  * 程序：以 `ps -eo pid,args` 依**腳本檔名**比對，對象是 `run-i074-stage2.sh`、`i074-stage2-supervisor.py`、`i074_stage2_sizing.py`（含 observer 的 `observe`）、`judge-i074-stage2.sh`、
    `finalize-stage2-evidence.sh`、`i074-stage2-sizing.sh`、`i074-stage2-acceptance.sh`、`run-replay-offline.sh`、`s11`／`s91`／`s92`。
    排除查詢本身；⛔ 不得有任何一個符合；⛔ 不用 `pgrep -f`／`pkill -f`。
  * 鎖：四個鎖檔逐一以 `flock -n` 確認沒有持有者。
* 記下每一項的 `du -sh` 與 `df -h /`。
* 只用**字面的絕對路徑** `rm -rf`（⛔ 不用變數、⛔ 不用 glob）；每刪一項就確認它已不存在。
* 使用者對某一項說「保留」，就照實記錄，⛔ 不刪。

## 五、主要風險與回滾

| 風險 | 處置 |
|---|---|
| 歸檔遺漏：移除之後找不到長期規格 | 階段 1 的逐節分類與關鍵值對照（「二」1、4）由 review 檢查；分成兩個 commit，階段 1 有問題不會牽動移除；最終仍可從 `20eacb2` 取回 |
| 斷鏈 | R1／R2 的改寫 ＋「六」的殘留掃描 ＋ `check-doc-refs.py` |
| 凍結集合被誤改，漂移測試失敗 | 「六」：`git diff --name-only` 不得包含凍結集合；完整跑 `python/scripts/test.sh`（含漂移測試） |
| 「只改註解」名不副實 | 「六」：Python 比對 AST（docstring 以外完全相同）；shell 與 TS 的每一個變更行，新舊兩側都是整行註解 |
| 基準線的殘留份額 | `check-doc-refs.py` 會擋下；依輸出遞減 |
| repo 外刪除無法還原 | 每一項先經使用者確認；evidence 在 git；image 與 tarball 保留 |
| 測試的資源（2 GiB host） | 一律用 repo 的腳本（mem-guard）；⛔ 不調大記憶體 |
| 回滾 | 文件與註解以 git 回復（兩個 commit 各自可 revert）；repo 外的刪除不可回復，所以放在最後、逐項確認 |

## 六、驗證

* 兩個階段都做：`python3 scripts/check-doc-refs.py`、`git diff --check`；新增的行裡「⛔」後面都帶否定詞；計畫書⛔ 不寫 `檔名:行號`。
* 階段 1：
  * 「二」4 的關鍵值對照全部找得到。
  * 「十二」的分類表涵蓋 I-074 的全部 76 節與前言，以及四份計畫書的每一節（v2）。
  * 收斂條件表的每一條都寫明滿足的方式（v2）。
* 階段 2：
  * 殘留掃描：
    * issue.md 檔頭的 `rg` 檢查：輸出裡的 I-074 只剩 R2、R3 與檔頭的收斂紀錄。
    * 指向 issue.md I-074、`#i-074` 錨點、`plans/i074-stage2-step*` 的引用：凍結集合與「〇」7 以外，每一處都帶 `20eacb2`（R2），其他為 0。
    * 沒有寫 I-074、只寫 `issue.md「Stage 2 …」`、`issue.md「③ …」` 這類的引用：凍結集合以外為 0。
  * 只改註解：
    * `git diff --name-only` 與凍結集合沒有交集。
    * 改過的 `.py`：與 HEAD 比對 AST（去掉 docstring）完全相同。
    * 改過的 `.sh` 與 `.ts`：每一個變更行，新舊兩側都是整行註解。
  * 測試：`python/scripts/test.sh` 全部通過，含 `scripts/test-replay-args.sh` 的漂移測試、`scripts/test-i074-stage2.sh`、`scripts/test-i074-step9.sh`；
    改過的 shell 先跑 `bash -n`；`srZones.ts` 跑 frontend 的測試（依 `development-workflow.md`）。

## 七、歸檔位置

`sr-zone-scoring.md`（契約、行為、正式值與錨、凍結集合舊引用的讀法）、`development-workflow.md`（作業程序）、issue.md 檔頭（收斂紀錄、凍結集合的例外）、
`todo.md`（T-077，依 D3）。

## 八、決定（⚠️ 待使用者裁決；v1 第一輪 review 建議 D1～D4 全部採 (a)）

| # | 決定 | 選項 | 理由與影響 |
|---|---|---|---|
| D1 | 凍結集合裡的註解（「〇」5；約 28 行） | **(a)** ⛔ 不改；在 issue.md 檔頭的殘留檢查註明例外，在 `sr-zone-scoring.md` 寫對照段，說明這些舊引用指的是哪些節、要到 `20eacb2` 讀<br>(b) 依 `development-workflow.md` 的版控流程，改 `evaluation.py` 與 `replay_bundle/` 的註解並重新產生 tooling patch；`decision_engine.py`、`lifecycle_engine.py` 仍然改不了（不變條件 ④ 會中止產生器），只能照 (a)<br>(c) 先退役漂移測試再改（屬驗證流程修改，另立計畫；本收斂延後） | ⚠️ 這一項與使用者「程式註解一起改」的裁決有出入，原因是盤點時才發現的限制。(a) 只多一個寫明的例外。(b) **在本計畫的範圍內⛔ 不可行**（v2 補正）：重新產生會改變 tooling patch 的 bytes 與 SHA，而現行的封存規則（`sr-zone-scoring.md`「I-074 Stage 2 的兩道 guard 與兩份 patch 的封存」）規定「封存之後任一份 patch 的 bytes 或 SHA 再變：⑨（兩道 guard ＋ 封存）、⑨-1、⑨-2 全部重做」，與「⛔ 不重跑任何 Stage」衝突；版控中的 patch 也會與 evidence 的封存副本（`5ff9a90f…`）不同。要走 (b) 必須先改封存規則，那屬於 (c) 的範圍。(c) 範圍與風險都大很多 |
| D2 | 歷史引用的寫法 | **(a)** 固定 commit：`git show 20eacb2:docs/issue.md` 的 I-074「…」<br>(b) 只寫「見 git 歷史」（`development-workflow.md` 已有這個先例） | (a) 可以直接取回，日後 issue.md 再改也⛔ 不會失準；(b) 讀者要自己找版本 |
| D3 | 凍結集合在 I-074 結束之後仍然有效 | **(a)** 階段 2 在 `todo.md` 登記 T-077「I-074 之後 Stage 2 工具與漂移測試的退役」：之後要改任何 Python 產品碼，都得先處理它；D1 留下的舊引用也在那時一起改<br>(b) 不登記 | 漂移測試的目的（⑩ 跑到正確的程式）已經達成，但測試仍然擋著所有 Python 產品碼的修改；不登記的話，下一個改 `decision_engine.py` 的人會在測試失敗時才發現 |
| D4 | 主題文件既有 I-074 節的篇幅 | **(a)** 只補缺、修引用與狀態，⛔ 不精簡<br>(b) 一併精簡（例如 ⑩ 的貼上版程序） | (a) 範圍可控；精簡可以等 D3 的退役一起做 |

## 九、執行步驟

1. 使用者確認本計畫（D1～D4）→ 使用者 commit 本檔。
2. 階段 1（「二」）→ 停下等 review → 使用者 commit。
3. 階段 2（「三」）→ 停下等 review → 「四」逐項確認並處置 → 使用者 commit。
4. 階段 2 review 確認之後，下一次收斂的 commit 刪除本檔。

## 十、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-10-29 | 初版（使用者裁決：⑪ review 通過、分兩階段、程式註解一起改（只改註解）、repo 外產物納入並逐項確認）；盤點時發現凍結集合，列為 D1、D3 |
| v2 | 2026-10-29 | v1 第一輪 review（建議補齊之後再確認；D1～D4 建議全部 (a)）的 3 項：<br>1. **高：刪除前的容器檢查不完整**——v1 只以 label 查容器，但 sizing／acceptance 的容器以 `i074sz-` 命名，⑪ 判讀器的驗證容器⛔ 沒有名稱與 label；工作仍存活時可能刪掉目錄或鎖檔。改成以名稱、label、指令列一起辨認，並補上 sentinel、`/dev/shm/i074-*` 與 `/dev/shm/s10-*`、程序（含 observer）與鎖的檢查；任一查詢失敗即停（「〇」14、「四」）。<br>2. **中：歸檔驗收沒有涵蓋即將刪除的四份計畫書**——v1 的分類只涵蓋 I-074 的 76 節，但階段 2 會刪四份計畫書，例如 ⑩「七」明文要求歸檔計次的更新。分類對象補上四份計畫書的每一節；另加收斂條件表，有任何一條⛔ 沒有滿足就⛔ 不進階段 2（「〇」13、「二」1、「六」）。<br>3. **中（只影響 D1(b)）：代價寫得不足**——現行封存規則要求任一份 patch 的 bytes 或 SHA 改變時，⑨、⑨-1、⑨-2 全部重做，與「⛔ 不重跑任何 Stage」衝突；只重新產生 patch 並不能完成。D1(b) 改寫成在本計畫範圍內⛔ 不可行（「八」） |
| v3 | 2026-10-29 | v2 第二輪 review 的 1 項（低）：**刪除程序的判定方式前後矛盾**——總則寫「下列任一項成立就停下」，但下方的 sentinel ⛔ 不存在、狀態目錄是空的、鎖拿得到都是**允許刪除**的條件。改成「沒有符合辨認條件的容器與程序、sentinel 與狀態目錄都不存在、四個鎖都拿得到，三者同時成立才可以刪；任一條件不符或查詢失敗就停下回報」；容器與程序兩項補上「⛔ 不得有任何一個符合」（「四」） |
