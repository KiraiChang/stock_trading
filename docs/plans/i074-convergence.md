# I-074 收斂計畫 v3（2026-10-29；✅ 確認（兩輪 review；D1～D4 採 (a)），commit `c35d4bf`；**階段 1 完成（「十二」，⚠️ 待 review；第一輪 review 的 3 項已修正；一項待使用者決定，見「十二」六）**；歷次修正見「十」）

需求確認（2026-10-29）：依 ⑪ 計畫 D3，另立本計畫，把 I-074 從 `docs/issue.md` 整筆收斂。使用者已裁決：

* ⑪ 的判讀結果（判讀 B）**review 通過**，可以排入「移除 I-074」。
* **分兩階段**：階段 1 歸檔到主題文件（review → commit）；階段 2 移除 I-074、修引用、刪計畫書（review → commit）。
* 程式與腳本裡指向 I-074 的註解**一起改、只改註解**。⚠️ 盤點時發現有一部分檔案被測試凍結、改了註解測試就會失敗，見「〇」5 與 D1。
* repo 外的產物**納入**本計畫；刪除放在最後一步，每一項刪除之前都先經使用者確認。

本計畫確認之前，⛔ 不改主題文件、⛔ 不改程式、⛔ 不刪任何檔案（唯一的例外是 I-074 狀態列加一行指向本檔）。✅ 2026-10-29 確認（commit `c35d4bf`）。

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

## 八、決定（✅ 使用者裁決 2026-10-29：D1～D4 全部採 (a)，與兩輪 review 的建議相同）

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
| v3（確認） | 2026-10-29 | 第二輪的修正 review 通過；使用者裁決 D1～D4 全部採 (a)（「八」）；commit `c35d4bf` |
| 階段 1（第一輪 review） | 2026-10-29 | 暫不通過，3 項（都是「十二」與主題文件的內容；計畫本身⛔ 沒有改）：<br>1. **中：C7 的前提不成立**——wf 的 `i074_stage2_acceptance_usage_001` 已有 ⑨-1 precheck 的完整副本（47 行、逐位元相同）；初稿只以區塊 id 搜尋而誤判。C7 改成已滿足，出處改成「⑨-1 執行紀錄格式 v2」的實作結果；6305 那一列補註樣板標題切開了格式 v2 的一節。<br>2. **中：最終的物理啟動次數沒有對帳**——4／8 沒有交代 2026-09-17 第一次 D 的 OOM，而計次政策寫明沒有產出的崩潰仍佔上限 ②。sr 改成兩種算法並列（4／8：⑩ 計畫的帳、依 v27 的格子定義；5／8：照字面計入），寫明依據與「⛔ 不影響判讀」。<br>3. **中：「`job_runs` 只留數天」不符現行程式**——程式常數 `jobRunRetentionDays` 是 30 天（自 `8051fe7`，2026-08-25）。訂正「十二」六與 wf「作業規則（正式量測）」；live 部署的版本未查證 |

## 十二、階段 1 的結果（2026-10-29，⚠️ 待 review）

簡寫：sr ＝ `sr-zone-scoring.md`、wf ＝ `development-workflow.md`。行號是 HEAD `c35d4bf` 的 `docs/issue.md` 與各計畫書裡的位置（文件裡的位置，⛔ 不是程式引用）。A ＝ 已歸檔、N ＝ 需補（本階段已補）、H ＝ 歷史（以 `20eacb2` 保存）。

### 一、I-074 的逐節分類（前言 ＋ 76 節）

| 行 | 行數 | 節 | 類別 | 依據 |
|---:|---:|---|---|---|
| 333 | 31 | （前言與欄位表） | A＋H | 問題陳述 → sr「已知並接受的行為改變」；狀態、嚴重度、來源隨條目移除；「已知限制（open）①」已移到 I-121 |
| 364 | 20 | replay 母體是 candles，不是 `stock_sr_zone_analyses`（2026-09-01 更正並收斂） | A | sr「已知並接受的行為改變」（replay 的母體是 candles 的更正）；wf「用真實資料跑 evaluation」 |
| 384 | 28 | 2026-09-01 replay 實測：零差異，但屬於「未觸發」 | A | sr「decision replay 實測（2026-09-01）」 |
| 412 | 18 | 「dev 沒有 model bundle」不是 blocker（2026-09-01 定案） | A＋H | live 唯讀、預設不寫 DB 與 CLAUDE.md 的界線 → wf「用真實資料跑 evaluation」；sr 已記「dev 沒有 model bundle」的理由是錯的；「通則」一段 → H |
| 430 | 310 | 處置（2026-09-01 定案）：一次有界定向驗證，結果決定收斂方式 | A＋H | 驗證目標與「證明不了的事」→ sr「I-074 定向驗證」的收斂措辭；寫死的範圍 → sr「I-074 Stage 1 的四個永久契約」；需要補的診斷欄位、candidate 的精確定義、責任層 → sr「Decision Replay 的診斷欄位」；四個階段、①② 的實際結果、before 版沒有 `lifecycle_engine.py`（v18 已取代）→ H |
| 740 | 14 | Stage 0 計畫書（v1～v17，2026-09-16 收斂） | H | 收斂紀錄；本節自述現況規格已歸檔到 sr |
| 754 | 59 | Stage 0 實作結果（2026-09-14） | A＋H | 九個診斷欄位與守門 → sr「Decision Replay 的診斷欄位」；改動檔案與測試結果 → H |
| 813 | 14 | Stage 1 計畫書（v1～v25，2026-09-16 收斂） | H | 收斂紀錄 |
| 827 | 173 | Stage 1 實作結果（2026-09-15） | A＋H | 四個永久契約 → sr；正式執行程序 → wf「I-074 Stage 1 的正式執行程序」；各輪 review → H |
| 1000 | 165 | ✅ 根因已定位並修正（2026-09-18）：`printf ／ grep -q` 在 `pipefail` 下的 SIGPIPE 競爭 | A＋N | **Stage 1 的正式結果 → N（已補）**：sr「I-074 Stage 1 的正式結果」；pipefail ＋ `grep -q` 的規則 → wf；分支 A 排除、第一次 D 的 OOM → wf「I-074 Stage 1 的正式執行程序」；comparator 的容量 → I-117；偶發失敗的調查經過 → H |
| 1165 | 34 | 修正（2026-09-17） | A＋H | 串流寫出的原因與數字 → wf（Stage 1 的 OOM 段）；改動檔案 → H（程式與測試本身保存） |
| 1199 | 176 | Stage 2 步驟 ⑦ 總綱 v1（2026-09-29，✅ **已確認**（2026-09-30，review 通過並 commit）） | A＋H | 本節的歸檔表各列 → sr「反事實 replay 路徑與 tooling patch」「正式證據契約」「晉升與 B／C 判讀器」、wf「tooling patch」「正式執行程序」；跨包介面 → H |
| 1375 | 273 | Stage 2 步驟 ⑦a 細部計畫 v1（2026-09-30，✅ **已確認**（2026-09-30，review 三輪後通過並 co | A＋H | sr「反事實 replay 路徑與 tooling patch」；wf「tooling patch」 |
| 1648 | 92 | Stage 2 步驟 ⑦a 實作結果（2026-09-30，✅ **review 通過**（兩輪修正後）並 commit `9e29658` | A＋H | 本節的歸檔表各列 → sr 三方 SHA、failed record、`--verified-*`；wf tooling patch 的版控流程與漂移測試；中間值、smoke、review → H |
| 1740 | 404 | Stage 2 步驟 ⑦b 細部計畫 v1（2026-09-30，✅ **已確認**（2026-09-30，review 三輪後通過並 co | A＋H | wf「正式執行程序」（orchestrator、supervisor、freeze record、preflight）；sr「晉升與 B／C 判讀器」 |
| 2144 | 94 | Stage 2 步驟 ⑦b 實作結果（2026-09-30，✅ **review 通過**（兩輪修正後）並 commit `ce17ab2` | H | 改動與測試；規格同上一節 |
| 2238 | 331 | Stage 2 步驟 ⑦c 細部計畫 v1（2026-10-01，✅ **已確認**（2026-10-01，review 七輪後通過並 co | A＋H | sr「晉升與 B／C 判讀器」（驗證入口、晉升的契約、B／C 規則、輸出 schema） |
| 2569 | 130 | Stage 2 步驟 ⑦c 實作結果（2026-10-01，✅ **review 通過**（四輪修正後）並 commit `5a03777` | A＋H | 「與計畫的差異」已併入 sr 晉升的契約（例如錯誤清理只刪自己的 inode）；其餘 → H |
| 2699 | 354 | Stage 2 步驟 ⑦d 細部計畫 v1（2026-10-02，✅ **已確認**（2026-10-02，review 六輪後通過並 co | A＋H | sr「容量驗收」；wf「acceptance harness 與 ⑩ 的 observer」 |
| 3053 | 153 | Stage 2 步驟 ⑦d 實作結果（2026-10-02，⚠️ **待 review**） | H | 規格同上一節 |
| 3206 | 300 | Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定（2026-10-05，v11，✅ **已確認**（2026-10-05， | A＋H | sr「容量驗收」的對象、精確閘與單向偵測器 |
| 3506 | 103 | Stage 2 步驟 ⑦d 增補的實作結果（2026-10-05，✅ **review 通過**（三輪，2026-10-06）並 commi | H | 規格同上一節 |
| 3609 | 164 | Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫（v8，2026-10-06，✅ **已確認**（review 七輪後  | A＋H | sr「容量驗收」；wf 的有效性條件與「作業規則（正式量測）」（本節的歸檔列） |
| 3773 | 78 | Stage 2 量測的有效性條件與 ⑨-1 fail-fast 的實作結果（2026-10-06，✅ **review 通過**（兩輪）並  | H | 規格同上一節 |
| 3851 | 229 | Stage 2 步驟 ⑧ 計畫 v4（2026-10-07，✅ **已確認**（review 三輪後 commit `5e2d84f`）） | H | 結果 → wf「I-074 Stage 2 的測試落點」 |
| 4080 | 364 | Stage 2 步驟 ⑧ 稽核結果（2026-10-07，✅ **確認點 ② 通過**（2026-10-07）） | A＋H | wf「I-074 Stage 2 的測試落點」（穩定的類別與入口、測試寫法）；逐 id 的稽核表 → H（wf 已寫明⛔ 不搬） |
| 4444 | 195 | Stage 2 步驟 ⑧ 補齊與全量執行的結果（2026-10-07～08，✅ **review 通過**（兩輪）並 commit `be0 | H | 「六」#1 已在 todo.md（現有項目的來源欄指向它，階段 2 改成歷史引用） |
| 4639 | 430 | Stage 2 步驟 ⑨ 細部計畫 v1（2026-10-08，✅ **review 通過（兩輪）並確認**） | A＋H | sr「兩道 guard 與兩份 patch 的封存」；wf「⑨：兩道 guard、封存與重新封存」 |
| 5069 | 197 | Stage 2 步驟 ⑨ 實作結果（2026-10-12，✅ **review 通過**（兩輪）並 commit `53538a3`） | A＋H | 封存值表 → sr（本節「6」裁決的封存形式） |
| 5266 | 268 | Stage 2 raw-failed freeze record 修正計畫 v1（2026-10-14，✅ 確認（兩輪 review）、co | A＋H | wf sizing 一節的 raw-failed freeze record 段；D5 → I-121 |
| 5534 | 127 | Stage 2 raw-failed freeze record 修正的實作結果（2026-10-14，✅ review 通過並 commi | H | 規格同上一節 |
| 5661 | 395 | Stage 2 步驟 ⑨-1 細部計畫 v8（2026-10-15，✅ 確認（七輪 review）並 commit `496a5f7`） | A＋H | 一次性的正式程序 → H（wf 指向本節「二」～「五」的引用，階段 2 改成歷史引用）；「十」的正式原始量測 → sr「容量驗收」 |
| 6056 | 122 | Stage 2 步驟 ⑨-1 前置修正（「十」）的 R0 與實作結果（2026-10-15，✅ review 通過、commit `ec23 | H | 修正已在程式；規格同上一節 |
| 6178 | 127 | Stage 2 步驟 ⑨-1 執行紀錄格式 v2（2026-10-16，✅ 確認（兩輪 review）；實作 ✅ review 通過（兩輪） | H | 本節自述樣板⛔ 不搬進主題文件 |
| 6305 | 276 | Stage 2 步驟 ⑨-1 正式驗收結果（<YYYY-MM-DD>，<結論>；⚠️ 待 review） | H | 結果的樣板。⚠️ 樣板本身是 `####` 標題，把「⑨-1 執行紀錄格式 v2」切開：這一段的後半其實是格式 v2 的「六」～「十」、兩輪 review 與實作結果（C5、C7 的出處） |
| 6581 | 477 | Stage 2 步驟 ⑨-1 的簡易指令 `s91` v5（2026-10-19，✅ 確認（五輪 review）並 commit `e3e4 | H | 本節自述工具只服務這一次、⛔ 不搬 |
| 7058 | 313 | Stage 2 步驟 ⑨-1 正式驗收結果（2026-10-24，⑨-1 通過；✅ review 通過並 commit `cbde340`） | A＋H | sr「容量驗收」的「⑨-1 的正式結果」；逐次嘗試與完整數字 → H |
| 7371 | 375 | Stage 2 步驟 ⑨-2 細部計畫 v4（2026-10-16，✅ 確認（四輪 review）並 commit `4f7e7f0`） | A＋H | 一次性的正式程序 → H；「保留的 S 的刪除程序」→ wf（已複製） |
| 7746 | 268 | Stage 2 步驟 ⑨-2 正式確認重跑結果（2026-10-27，⑨-2 通過；✅ review 通過並 commit `62e1773 | A＋H | sr「容量驗收」的「⑨-2 的正式結果」（`P_B`、freeze record） |
| 8014 | 209 | I-074 Stage 2 計畫書 v29（2026-09-29，步驟 ⑥，✅ **已確認**（2026-09-29，review 通過並  | A＋N | sr Stage 2 各節、wf「正式執行程序」；**磁碟預算 → N（已補）**：sr「容量驗收」的「磁碟預算與可用空間門檻」 |
| 8223 | 27 | （承上）I-074 Stage 2 計畫書 v28（2026-09-23，✅ **已確認**）的內容 | H | 修訂紀錄（現行內容已併入 v29） |
| 8250 | 16 | （承上）Stage 2 計畫書 v27（2026-09-23）的內容 | H | 同上 |
| 8266 | 33 | （承上）Stage 2 計畫書 v26（2026-09-23）的內容 | H | 同上 |
| 8299 | 12 | （承上）Stage 2 計畫書 v25（2026-09-22，✅ **已確認**）的收尾 | H | 同上 |
| 8311 | 23 | （承上）Stage 2 計畫書 v24 的缺口修補 | H | 同上 |
| 8334 | 18 | （承上）Stage 2 計畫書 v23 的缺口修補 | H | 同上 |
| 8352 | 17 | （承上）Stage 2 計畫書 v22 的缺口修補 | H | 同上 |
| 8369 | 21 | （承上）Stage 2 計畫書 v21 的缺口修補 | H | 同上 |
| 8390 | 17 | （承上）Stage 2 計畫書 v20 的缺口修補 | H | 同上 |
| 8407 | 12 | （承上）Stage 2 計畫書 v19 的缺口修補 | H | 同上 |
| 8419 | 1332 | （承上）Stage 2 計畫書 v18 的整合內容 | A＋H | v3 比較模型 → sr「反事實 replay 路徑與 tooling patch」；判讀矩陣 → sr 判讀器；環境見證 → sr／wf；本節的歸檔表各列已在主題文件 |
| 9751 | 149 | ② before tooling／反事實謂詞設計 v2（2026-09-22；✅ **2026-09-23 review 通過**，見「②  | A＋H | counterfactual 謂詞 → sr「反事實 replay 路徑與 tooling patch」 |
| 9900 | 126 | ② 執行結果（2026-09-22；✅ **2026-09-23 review 通過**，見「② 的 review 結論」） | H | 執行紀錄 |
| 10026 | 11 | ③ Stage 2 evidence contract 計畫書 v12（2026-09-23，✅ **已確認**） | H | 版本頭 |
| 10037 | 10 | （承上）③ evidence contract 計畫書 v11（2026-09-23）的內容 | H | 修訂紀錄 |
| 10047 | 17 | （承上）③ evidence contract 計畫書 v10（2026-09-23）的內容 | H | 同上 |
| 10064 | 9 | （承上）③ evidence contract 計畫書 v9 的內容 | H | 同上 |
| 10073 | 10 | （承上）③ evidence contract 計畫書 v8 的內容 | H | 同上 |
| 10083 | 9 | （承上）③ evidence contract 計畫書 v7 的內容 | H | 同上 |
| 10092 | 9 | （承上）③ evidence contract 計畫書 v6 的內容 | H | 同上 |
| 10101 | 10 | （承上）③ evidence contract 計畫書 v5 的內容 | H | 同上 |
| 10111 | 11 | （承上）③ evidence contract 計畫書 v4 的內容 | H | 同上 |
| 10122 | 14 | （承上）③ evidence contract 計畫書 v3 的內容 | H | 同上 |
| 10136 | 16 | （承上）③ evidence contract 計畫書 v2 的內容 | H | 同上 |
| 10152 | 858 | （承上）③ evidence contract 計畫書 v1 的內容 | A＋H | sr「正式證據契約」「環境見證契約」（layout、manifest、三方 SHA、記憶體模型、failed record）；wf「finalizer 的 CLI matrix」「正式證據與 failed record 程序」；本節的歸檔表各列已在主題文件 |
| 11010 | 62 | ③b 實作結果（2026-09-23，✅ **review 通過**） | A＋H | wf「環境見證程序」 |
| 11072 | 35 | ③c 準備：Stage 2 image 的來源（2026-09-23，✅ `--adopt-image` review 通過） | A＋H | wf「環境見證程序」（採用 image 的條件、專用 tag 與 tarball） |
| 11107 | 30 | ③c 執行結果（2026-09-23，✅ **EQUIVALENT**） | N | **N（已補）**：sr「環境見證契約」的「③c 的正式結果」 |
| 11137 | 89 | ③d 實作結果（2026-09-24，✅ **review 通過**） | A＋H | sr「正式證據契約」；wf「正式證據與 failed record 程序」 |
| 11226 | 361 | Stage 2 步驟 ④：sizing harness 計畫書 v7（2026-09-24，✅ **已確認**） | A＋H | wf「sizing harness」；sr「容量驗收」 |
| 11587 | 140 | Stage 2 步驟 ④ 實作結果（2026-09-24，✅ **review 通過**——2026-09-29，四輪——並 commit） | H | 規格同上一節 |
| 11727 | 75 | Stage 2 步驟 ⑤：`--formal` 正式量測與 `M_safety` 的裁定（2026-09-29 完成，✅ **review  | N | **N（已補）**：sr「容量驗收」的「磁碟預算與可用空間門檻」「⑤ 的正式量測」 |
| 11802 | 54 | Stage 2 的前置盤點（2026-09-18，⚠️ **計畫書的材料，⛔ 不是計畫書本身**） | H | 本節自述是計畫書的材料 |
| 11856 | 45 | 正式 scan 的計次裁決（2026-09-11 使用者明文確認） | H＋N | 政策 → H；**最終計次 → N（已補）**：sr「⑩ 的正式值」 |
| 11901 | 167 | 關閉條件（2026-09-01 改為單一決策樹） | A＋H | 判讀矩陣、分類欄位、不可觀測欄位、AVOID 短路 → sr 判讀器的「B／C 判讀規則」；結論 → sr「I-074 定向驗證」；分支 A 的收斂措辭（未走到）→ H |
| 12068 | 30 | 前置：I-100 的可重現路徑（硬性，Stage 1 前必須完成） | H | I-100 是獨立條目 |
| 12098 | 37 | 測試、風險與歸檔 | A＋H | 測試落點 → wf；candidate 的守門 → sr「Decision Replay 的診斷欄位」；歸檔要求已由 ⑪ 完成（sr「I-074 定向驗證」）；護欄 → H |

共 77 列（前言 1 ＋ 76 節），每一列都有分類。

### 二、四份計畫書的逐節分類

`i074-stage2-step10.md`：

| 行 | 節 | 類別 | 依據 |
|---:|---|---|---|
| 2 | （前言與欄位表） | H | 標題與需求 |
| 14 | 〇、唯讀盤點（2026-10-27，HEAD `62e1773`） | H | — |
| 31 | 一、目標、範圍與⛔ 不做 | H | — |
| 39 | 二、前置條件（每一次開跑——含重跑——都逐項確認、記進結果；任一項不成立就⛔ 不開跑） | A＋H | wf「⑩ 的正式執行程序」的 precheck 區塊 |
| 132 | 三、啟動、程序身分、狀態、安全中止與完成判定（D1：沿用 ⑨-1 的 wrapper，由使用者在 tmux／自己的終端機執行） | A＋H | wf「⑩ 的正式執行程序」的啟動、續跑區塊與控制目錄、observer 的表 |
| 318 | 四、判讀與分支（一律依 rc ＋ `<work>/state/` ＋ 真正 repo 的磁碟事實；每一次嘗試都記進結果） | A | wf「⑩ 的正式執行程序」的判讀表 |
| 358 | 五、驗證與保存（rc 0 時全做；其他結果做適用的部分） | A | wf「⑩ 的正式執行程序」的「跑完之後」 |
| 379 | 六、交給 ⑪ 的規則 | A | wf「跑完之後」最後一點；sr 判讀器 |
| 386 | 七、歸檔與 commit | 收斂條件 | 見收斂條件表 C1 |
| 396 | 八、風險 | H | — |
| 415 | 九、決定（✅ D1～D9 已定案，2026-10-28） | H | — |
| 431 | 十、執行步驟 | H | — |
| 452 | 十一、修訂紀錄 | H | — |
| 464 | 十二、review 的收斂條件（v3，✅ 使用者裁決 2026-10-28） | H | — |
| 476 | 十三、實作結果（「十」2，2026-10-28，✅ review 通過（第四輪），commit `a9cb41d`） | H | — |
| 487 | 十四、執行結果（⑩，2026-10-28，⑩ 成功；⚠️ 已執行／待 review） | A＋N＋H | 「一」交給 ⑪ 的值 → sr「⑩ 的正式值」；**「六」計次 → N（已補）**；「七」觀察 → I-120、I-116；其餘 → H |

`i074-stage2-step11.md`：

| 行 | 節 | 類別 | 依據 |
|---:|---|---|---|
| 2 | （前言與欄位表） | H | 標題與需求 |
| 12 | 〇、唯讀盤點（2026-10-29，HEAD `078ec72`） | H | — |
| 30 | 一、目標、範圍與⛔ 不做 | H | — |
| 38 | 二、前置條件（第二段的腳本**逐項自動判定，任一項不成立就停止**、⛔ 不呼叫判讀器） | H | s11 的 J1～J7；批次的作業規則本身 → wf「作業規則（正式量測）」 |
| 54 | 三、執行（D1：第一段由使用者、第二段由 Claude；v4 起兩段都用簡易指令 `s11`；輸出保存依 D2） | H | — |
| 74 | 三之一、腳本（`~/.local/bin/s11`） | H | 工具只服務這一次（比照 s91、s92） |
| 214 | 三之二、核對指令（`s11 status` 之前，使用者執行；一行） | H | — |
| 223 | 四、輸出驗證與判讀（依序；任一項不符就停下回報） | H | — |
| 236 | 五、下游影響的摘要（D5；B、C 都做；判讀之後、只描述、⛔ 不參與 B／C 的判定） | A | sr「I-074 定向驗證」的表 |
| 250 | 六、分支處置 | A | 收斂措辭 → sr「I-074 定向驗證」（原樣引用） |
| 265 | 七、紀錄、歸檔與 commit | 收斂條件 | 見收斂條件表 C2 |
| 273 | 八、風險 | H | — |
| 291 | 九、決定（✅ 使用者裁決 2026-10-29：D1～D5 全部採 (a)，與兩輪 review 的建議相同） | H | D2 的輸出保存 → 本計畫「四」 |
| 308 | 十、執行步驟 | H | — |
| 321 | 十一、修訂紀錄 | H | — |
| 333 | 十二、「十」2、3 的結果（v4，2026-10-29，⚠️ 待 review） | H | — |
| 346 | 十三、執行結果（2026-10-29，判讀 **B**；⚠️ 已執行／待 review） | A＋H | 「一」→ sr「⑪ 的正式值」；**「八」觀察 1（2026-10-29 live 批次的失敗與 partial）⚠️ 沒有歸檔，待使用者決定（見「十二」六）**；觀察 2 → I-120；其餘 → H |

`i074-stage2-step92-p1.md`：

| 行 | 節 | 類別 | 依據 |
|---:|---|---|---|
| 2 | （前言與欄位表） | H | 標題與需求 |
| 9 | 〇、唯讀盤點（2026-10-27，HEAD `cbde340`） | H | — |
| 20 | 一、目標、範圍與⛔ 不做 | H | — |
| 28 | 二、設計 | N | **N（已補）**：「文件」的定義 → wf acceptance／sizing 的表 |
| 42 | 三、測試與驗證（實作之後；⛔ 不在真正的 repo 建 commit） | H | — |
| 51 | 四、風險與回滾 | H | — |
| 60 | 五、決定 | H | — |
| 68 | 六、歸檔與執行步驟 | 收斂條件 | 見收斂條件表 C3 |
| 74 | 七、實作結果（2026-10-27，待 review） | H | — |

`i074-stage2-step92-s92.md`：

| 行 | 節 | 類別 | 依據 |
|---:|---|---|---|
| 2 | （前言與欄位表） | H | 標題與需求 |
| 7 | 〇、唯讀盤點（2026-10-27，HEAD `eb855b0`） | H | — |
| 20 | 一、目標、範圍與⛔ 不做 | H | — |
| 28 | 二、設計（`s91` v5 的骨架，換成 ⑨-2 的區塊） | H | — |
| 40 | 三之一、腳本（`~/.local/bin/s92`） | H | 工具只服務這一次 |
| 248 | 三之二、核對指令（`s92 pre` 之前，使用者執行；一行） | H | — |
| 256 | 四、決定 | H | — |
| 265 | 五、受影響的檔案與歸檔 | 收斂條件 | 見收斂條件表 C4 |
| 275 | 六、風險與回滾 | H | — |
| 287 | 七、測試與驗證（確認之後；scratchpad 的測試副本，`HOME` 指到 scratchpad，⛔ 不在真正的 repo 跑 `go` | H | — |
| 297 | 八、執行步驟 | H | — |
| 304 | 九、實作結果（2026-10-27，待 review） | H | — |
| 325 | 第一輪 review 的修正（2026-10-27） | H | — |
| 334 | 十、實際使用（2026-10-27，⑨-2；✅ review 通過並 commit `62e1773`） | A＋H | ⑨-2 的結果 → sr「容量驗收」 |


### 三、收斂條件表（I-074 內各計畫節與四份計畫書自己寫的收斂條件或歸檔要求）

| # | 出處 | 條件 | 滿足的方式 | 狀態 |
|---|---|---|---|---|
| C1 | `i074-stage2-step10.md`「七」 | 刪除之前，「十四」裡的長期內容——交給 ⑪ 的錨、計次的更新、嘗試的結論——已歸檔 | 錨 → sr「⑩ 的正式值」（原有）；計次與嘗試 → 同一張表新增的「正式 scan 的計次（最終）」「嘗試」兩列 | ✅ |
| C2 | `i074-stage2-step11.md`「七」、D2 | 與 ⑩、⑨-1、⑨-2 的計畫一起收斂；判讀器的輸出保存到 I-074 收斂 | 計畫書 → 階段 2 刪除；輸出 → 本計畫「四」 | ✅（計畫內） |
| C3 | `i074-stage2-step92-p1.md`「六」3 | P1 的現況規格（「文件」的定義）搬進 wf 的 acceptance／sizing 一節 | wf acceptance 的表新增「正式量測之間『只動文件』的定義」列 | ✅ |
| C4 | `i074-stage2-step92-s92.md`「五」 | 工具只服務這一次、⛔ 不搬進主題文件；隨 ⑨-2 的計畫收斂 | H；`s92` 本身 → 本計畫「四」 | ✅ |
| C5 | I-074「⑨-1 執行紀錄格式 v2」「六」、D6 | 隨 ⑨-1 的計畫收斂；樣板⛔ 不搬；收斂之後 `grep -rn '執行紀錄格式' docs/` 只剩附 commit 的引用 | H；階段 2 的殘留掃描加上這一條 | ✅（階段 2 驗） |
| C6 | I-074「s91 v5」 | 工具只服務這一次、⛔ 不搬 | H；`s91` 本身 → 本計畫「四」 | ✅ |
| C7 | I-074「⑨-1 執行紀錄格式 v2」的實作結果（2026-10-16）、與計畫的差異 3 | 「隨 ⑨-1 的計畫一起收斂之後，只剩 `development-workflow.md` 的副本」（⑨-1 的 precheck 區塊 `i074_stage2_step91_precheck_002`） | wf 的 `i074_stage2_acceptance_usage_001` 區塊裡**已有完整副本**：47 行，與 issue.md 的兩份逐位元相同（`20eacb2`、`f027cb2` 的版本也都有）。保留既有副本，⛔ 不新增、⛔ 不改寫。（初稿只以區塊 id 搜尋，誤判成沒有副本；階段 1 第一輪 review 指正） | ✅ |
| C8 | I-074「raw-failed freeze record 修正計畫 v1」D5 | 「已知限制（open）①」⛔ 不隨該節收斂 | 已移到 I-121 | ✅ |
| C9 | I-074 的四張「歸檔到」表（⑦ 總綱、⑦a 實作結果、v18（v26 補）、③ v1（v10）） | 每一列的內容在指定的主題文件 | 逐列查過：sr 的三方 SHA、failed record、`--verified-*`、v3 比較模型、判讀矩陣與規則、環境見證、layout 與 manifest；wf 的 tooling patch 版控流程與漂移測試、正式執行程序、failed record 程序、「Stage 2 archive 必須與 Stage 1 evidence 一起保存」、CLI matrix、專用 tag 與 tarball、Stage 1 的 OOM 與串流寫出。v18 歸檔表的「串流比對的理由與峰值數字 → wf」實際寫在 sr「環境見證契約」的記憶體模型（135 MiB 對 449 MiB），內容在、位置不同，⛔ 不搬動。「B／C 判定結果與證據 SHA → issue.md I-074」那一列，現在在 sr「I-074 定向驗證」 | ✅ |
| C10 | I-074「⑨ 細部計畫」決定 6 | 封存值寫進結果並歸檔到 sr | sr「兩道 guard 與兩份 patch 的封存」的封存值表 | ✅ |
| C11 | I-074「有效性條件與 ⑨-1 fail-fast 計畫」 | 作業規則歸檔到 wf | wf「作業規則（正式量測）」 | ✅ |
| C12 | I-074「處置」 | 收斂時的措辭必須守住「⛔ 不能推論自然母體的發生率」 | sr「I-074 定向驗證」的措辭原樣 | ✅ |
| C13 | I-074「測試、風險與歸檔」 | 驗收結論與仍需保留的限制寫進 sr，本筆經 review 確認後再移除 | 結論 → sr「I-074 定向驗證」；仍 open 的限制都是獨立條目（I-115～I-121） | ✅ |

### 四、關鍵值對照（`grep -cF`，sr／wf 各自出現的行數；兩者都是 0 才算缺）

| 階段 | 值 | 內容 | sr | wf |
|---|---|---|---:|---:|
| Stage 1 | evidence_manifest.json 的 SHA | `485fb6019d77…` | 1 | 0 |
| Stage 1 | D+1 after artifact（payload） | `33a6b1666487…` | 2 | 0 |
| Stage 1 | D+1 after artifact（stored） | `8cab461c0fe2…` | 1 | 0 |
| Stage 1 | 仲裁 | `MATCH` | 5 | 4 |
| ③c | 採用的 image | `2a90ad1c1dd8…` | 1 | 8 |
| ③c | pip_freeze_sha256 | `7a39573ee051…` | 1 | 0 |
| ③c | witness after | `f5d6d9eaf2ca…` | 1 | 0 |
| ③c | cohort' | `d5768e8fe184…` | 1 | 0 |
| ③c | envcheck 進版控 | `5bae980` | 1 | 0 |
| ⑤ | P_B | `158,101,504` | 1 | 0 |
| ⑤ | M_safety | `1,073,741,824` | 1 | 0 |
| ⑤ | P_B_BUDGET | `167,772,160` | 3 | 0 |
| ⑤ | REQUIRED_BYTES | `1,241,513,984` | 1 | 1 |
| ⑨ | counterfactual | `fa7f5ba840a3…` | 1 | 0 |
| ⑨ | tooling | `5ff9a90fe331…` | 1 | 0 |
| ⑨ | composed | `ac49d36ff07a…` | 1 | 0 |
| ⑨ | counterfactual 語意 | `5b851e1eef40…` | 1 | 0 |
| ⑨ | 驗證時的 source tree | `c40162551af9…` | 1 | 0 |
| ⑨-1 | success 的 P_basis | `162,926,592` | 1 | 0 |
| ⑨-1 | raw manifest | `cfe94e0a0674…` | 1 | 0 |
| ⑨-1 | repo_head | `d3df284` | 1 | 0 |
| ⑨-2 | P_B | `163,024,896` | 1 | 0 |
| ⑨-2 | freeze record | `098a39363d7b…` | 1 | 1 |
| ⑨-2 | repo_head | `832f678` | 3 | 1 |
| ⑩ | evidence 檔案清單的錨 | `2da23d3aa83c…` | 2 | 0 |
| ⑩ | evidence_manifest.json 的 SHA | `d5cf06298b4d…` | 1 | 0 |
| ⑩ | base_commit | `832f678acfa4…` | 1 | 1 |
| ⑩ | before artifact | `6f25b2d8fa85…` | 2 | 0 |
| ⑩ | comparison artifact | `a39cfbc8050d…` | 2 | 0 |
| ⑩ | report artifact | `d00ab7cf7693…` | 1 | 0 |
| ⑩ | identity | `a02bf3bd` | 1 | 3 |
| ⑩ | 計次（最終） | `4／5` | 1 | 0 |
| ⑪ | verdict.json | `ac2f393c025a…` | 2 | 0 |
| ⑪ | 判讀 | `判讀 **B**` | 1 | 1 |
| ⑪ | 持倉建議實際改變 | `71` | 14 | 4 |

共 35 個值，⛔ 沒有兩者都是 0 的。

### 五、主題文件的變更（本階段）

* **sr**：
  * 新增「I-074 Stage 1 的正式結果」：D、D+1、仲裁、「provenance 相同」的精確意思、證據位置、manifest 與 D+1 的 SHA、決策樹。
  * 「環境見證契約」新增「③c 的正式結果」：採用的 image、pip freeze、等價判定、信任錨、獨立複核、含意。
  * 「容量驗收」新增「磁碟預算與可用空間門檻」（`P_B_BUDGET`、`M_safety` 的裁定與理由、`REQUIRED_BYTES`）與「⑤ 的正式量測」。
  * 「⑩ 的正式值」新增「嘗試」與「正式 scan 的計次（最終）」兩列。
  * 狀態標記校正：「I-074 定向驗證」→ ✅ `20eacb2`；「兩道 guard 與兩份 patch 的封存」→ ✅ 兩輪、`53538a3`；「晉升與 B／C 判讀器」標題與「⑩ 的正式值」→ ✅ `078ec72`；⑪ → ✅ `20eacb2`。
* **wf**：
  * acceptance 的表新增「正式量測之間『只動文件』的定義」（C3）。
  * 狀態標記校正：「⑨：兩道 guard、封存與重新封存」→ ✅ 兩輪、`53538a3`；「I-074 Stage 2 的正式執行程序」標題與「⑩ 的正式執行程序」→ ✅ `078ec72`；⑪ 的指標 → ✅ `20eacb2`。
* 依據：⑨ 的 review 與 commit 見 I-074 的狀態列；⑩ 由使用者 review 之後 commit `078ec72`；⑪ 的 review 通過見本計畫的需求確認、commit `20eacb2`。
* 階段 1 第一輪 review 的修正：sr「⑩ 的正式值」的計次改寫成兩種算法並寫明依據（上限 ② 是 4／8 或 5／8，差在 2026-09-17 的 OOM）；wf「作業規則（正式量測）」的「`job_runs` 只留數天」訂正為程式常數 30 天（live 部署的版本未查證）。
* ⛔ 沒有改 I-074、⛔ 沒有改程式；文件裡仍指向 issue.md I-074 的引用都留到階段 2。

### 六、待使用者決定（⚠️ 決定之前⛔ 不進階段 2）

1. **⑪ 計畫「十三」八、觀察 1**：2026-10-29 當天 live 的 `delisting_reconcile` failed、`intraday` 13:30 failed、`stock_symbol_sync` 與 `corporate_action_sync` partial（前一天都是 success）。這不屬於 I-074，計畫書刪除之後只剩 git 歷史。
   * (a)（階段 1 第一輪 review 建議）：另立 issue 調查。先保存當天 `job_runs` 的錯誤、時間與失敗數，再判斷原因；partial 本身⛔ 不等於程式缺陷，`delisting_reconcile` 的失敗也可能是上游 `stock_symbol_sync` 沒有成功所致。
     查 live 只能唯讀，JWT 與 DB 連線由使用者在自己的終端機執行（⛔ 不經過 Claude）。`job_runs` 的保留期是程式常數 30 天（`jobRunRetentionDays`；live 部署的版本⛔ 未查證）。
   * (b)：只把觀察記進 issue.md 一筆（待查）。
   * (c)：不處理。

### 七、驗證（本階段）

* `python3 scripts/check-doc-refs.py`：問題 0；`git diff --check`：通過；新增的行裡「⛔」後面都帶否定詞；本檔⛔ 沒有 `檔名:行號`。
* 分類表涵蓋 I-074 的 77 列（前言 ＋ 76 節）與四份計畫書的 56 節（`conv-s12.py` 對每一節斷言有分類）。
* 關鍵值 35 個都在主題文件裡找得到（「四」）。
* C7 的重驗：從 issue.md 抽出兩份 `i074_stage2_step91_precheck_002`（各 47 行、逐位元相同），確認它是 wf `i074_stage2_acceptance_usage_001` 區塊的逐位元子字串。
* `git status`：只改 `docs/sr-zone-scoring.md`、`docs/development-workflow.md`、本檔與 `docs/issue.md`（I-074 狀態列裡指向本檔的那一行）。
